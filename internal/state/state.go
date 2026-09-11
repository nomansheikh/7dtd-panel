// Package state polls the game server and keeps the last known good answer.
//
// Two rules drive the design. The panel must keep serving when the game server
// is unreachable, and a single failed poll must not blank the UI or flip the
// status to offline. So every successful poll is cached with the time it was
// taken, failures leave the cache untouched, and "offline" requires several
// consecutive failures rather than one.
package state

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/nomansheikh/7dtd-panel/internal/sdtd"
)

// Status is the panel's view of the game server.
type Status string

const (
	// StatusUnknown means no poll has completed yet.
	StatusUnknown Status = "unknown"
	// StatusOnline means the most recent poll succeeded.
	StatusOnline Status = "online"
	// StatusDegraded means recent polls failed but not enough to call it down.
	// Cached data is still served, flagged with its age.
	StatusDegraded Status = "degraded"
	// StatusOffline means failures reached the configured threshold.
	StatusOffline Status = "offline"
)

// GameClient is the slice of the sdtd client the poller needs. It exists so
// tests can drive failure sequences without an HTTP server.
type GameClient interface {
	ServerStats(ctx context.Context) (sdtd.ServerStats, error)
	ServerInfo(ctx context.Context) (sdtd.ValueSet, error)
	Bloodmoon(ctx context.Context) (sdtd.Bloodmoon, error)
	Log(ctx context.Context, firstLine, count int) (sdtd.LogPage, error)
}

// Snapshot is an immutable view of what the panel currently believes.
//
// Each section carries its own timestamp, because they refresh at different
// rates and the UI shows staleness rather than hiding it.
type Snapshot struct {
	Status              Status
	ConsecutiveFailures int
	// LastError is the most recent poll failure, already safe to show to a
	// logged-in operator.
	LastError string

	Stats   sdtd.ServerStats
	StatsAt time.Time

	Version        string
	World          string
	GameMode       string
	CurrentPlayers int
	MaxPlayers     int
	InfoAt         time.Time

	// Uptime is the game server's uptime as of UptimeSampledAt. It comes from
	// the newest log line's uptime field, which is the only place the API
	// exposes uptime at all.
	Uptime          time.Duration
	UptimeSampledAt time.Time

	Bloodmoon   sdtd.Bloodmoon
	BloodmoonAt time.Time
}

// Reachable reports whether the server is believed to be answering.
func (s Snapshot) Reachable() bool {
	return s.Status == StatusOnline || s.Status == StatusDegraded
}

// StatsAge reports how old the cached stats are, and whether there are any.
func (s Snapshot) StatsAge(now time.Time) (time.Duration, bool) {
	if s.StatsAt.IsZero() {
		return 0, false
	}
	return now.Sub(s.StatsAt), true
}

// UptimeAt extrapolates uptime to now, so the dashboard ticks up between polls
// instead of jumping once a minute.
func (s Snapshot) UptimeAt(now time.Time) (time.Duration, bool) {
	if s.UptimeSampledAt.IsZero() {
		return 0, false
	}
	d := s.Uptime + now.Sub(s.UptimeSampledAt)
	if d < 0 {
		return 0, false
	}
	return d, true
}

// Options configures a Poller.
type Options struct {
	Client GameClient
	// Interval is the base cadence; the cheap stats call runs every tick.
	Interval time.Duration
	// FailureThreshold is how many consecutive stats failures are needed
	// before the status becomes offline.
	FailureThreshold int
	Logger           *slog.Logger
	// Now is overridable for tests.
	Now func() time.Time
}

// Poller refreshes the snapshot in the background. It is safe for concurrent
// use once constructed.
type Poller struct {
	client    GameClient
	interval  time.Duration
	threshold int
	log       *slog.Logger
	now       func() time.Time

	mu   sync.RWMutex
	snap Snapshot

	// Resources other than stats refresh more slowly; serverinfo is ~5 KB and
	// changes rarely, so polling it every few seconds is pure waste.
	infoEvery      time.Duration
	bloodmoonEvery time.Duration
	uptimeEvery    time.Duration
}

// New builds a Poller. It does not start polling; call Run.
func New(opts Options) *Poller {
	interval := opts.Interval
	if interval <= 0 {
		interval = 5 * time.Second
	}
	threshold := opts.FailureThreshold
	if threshold < 1 {
		threshold = 3
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	return &Poller{
		client:         opts.Client,
		interval:       interval,
		threshold:      threshold,
		log:            logger,
		now:            now,
		snap:           Snapshot{Status: StatusUnknown},
		infoEvery:      60 * time.Second,
		bloodmoonEvery: 30 * time.Second,
		uptimeEvery:    30 * time.Second,
	}
}

// Snapshot returns the current view. The returned value is a copy and safe to
// hold.
func (p *Poller) Snapshot() Snapshot {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.snap
}

// Run polls until ctx is cancelled. It polls once immediately so the dashboard
// is not blank for a whole interval on startup.
func (p *Poller) Run(ctx context.Context) {
	p.Tick(ctx)

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.log.Debug("poller stopping")
			return
		case <-ticker.C:
			p.Tick(ctx)
		}
	}
}

// Tick performs one polling round. Exported so tests can step deterministically
// rather than waiting on wall-clock timers.
func (p *Poller) Tick(ctx context.Context) {
	p.pollStats(ctx)

	// The slower resources are only worth fetching when the server is
	// answering at all; hammering a down server achieves nothing.
	if !p.Snapshot().Reachable() {
		return
	}
	now := p.now()
	if p.due(p.snapshotInfoAt(), p.infoEvery, now) {
		p.pollInfo(ctx)
	}
	if p.due(p.snapshotUptimeAt(), p.uptimeEvery, now) {
		p.pollUptime(ctx)
	}
	if p.due(p.snapshotBloodmoonAt(), p.bloodmoonEvery, now) {
		p.pollBloodmoon(ctx)
	}
}

func (p *Poller) due(last time.Time, every time.Duration, now time.Time) bool {
	return last.IsZero() || now.Sub(last) >= every
}

func (p *Poller) snapshotInfoAt() time.Time {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.snap.InfoAt
}

func (p *Poller) snapshotUptimeAt() time.Time {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.snap.UptimeSampledAt
}

func (p *Poller) snapshotBloodmoonAt() time.Time {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.snap.BloodmoonAt
}

// pollStats is the health signal. /api/serverstats is ~150 bytes, making it
// the cheapest way to ask whether the server is alive.
func (p *Poller) pollStats(ctx context.Context) {
	stats, err := p.client.ServerStats(ctx)

	p.mu.Lock()
	defer p.mu.Unlock()

	if err != nil {
		p.snap.ConsecutiveFailures++
		p.snap.LastError = safeError(err)

		switch {
		case p.snap.StatsAt.IsZero():
			// Never succeeded, so there is no good state to protect.
			p.snap.Status = StatusOffline
		case p.snap.ConsecutiveFailures >= p.threshold:
			p.snap.Status = StatusOffline
		default:
			p.snap.Status = StatusDegraded
		}

		p.log.Warn("game server poll failed",
			"consecutiveFailures", p.snap.ConsecutiveFailures,
			"status", string(p.snap.Status),
			"error", err)
		// Deliberately leaves Stats and StatsAt alone: the cached answer is
		// still the best one available.
		return
	}

	was := p.snap.Status
	p.snap.ConsecutiveFailures = 0
	p.snap.LastError = ""
	p.snap.Status = StatusOnline
	p.snap.Stats = stats
	p.snap.StatsAt = p.now()

	if was != StatusOnline {
		p.log.Info("game server reachable", "previousStatus", string(was))
	}
}

func (p *Poller) pollInfo(ctx context.Context) {
	info, err := p.client.ServerInfo(ctx)
	if err != nil {
		p.log.Warn("serverinfo poll failed", "error", err)
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	// ServerVersion is the server's own rendering and differs from what the
	// console version command reports for the same build. Using it anyway
	// avoids executing a command, which would write a line into the log feed
	// on every poll.
	p.snap.Version = info.Str("ServerVersion")
	p.snap.World = info.Str("LevelName")
	p.snap.GameMode = info.Str("GameMode")
	p.snap.CurrentPlayers = int(info.Int("CurrentPlayers"))
	p.snap.MaxPlayers = int(info.Int("MaxPlayers"))
	p.snap.InfoAt = p.now()
}

// pollUptime reads the newest log line purely for its uptime field. Nothing
// else in the API reports how long the server has been up.
func (p *Poller) pollUptime(ctx context.Context) {
	page, err := p.client.Log(ctx, -1, -1)
	if err != nil {
		p.log.Warn("uptime poll failed", "error", err)
		return
	}
	if len(page.Entries) == 0 {
		return
	}
	newest := page.Entries[len(page.Entries)-1]
	d, ok := newest.UptimeDuration()
	if !ok {
		p.log.Warn("log entry has unparseable uptime", "uptime", newest.Uptime)
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.snap.Uptime = d
	p.snap.UptimeSampledAt = p.now()
}

func (p *Poller) pollBloodmoon(ctx context.Context) {
	bm, err := p.client.Bloodmoon(ctx)
	if err != nil {
		p.log.Warn("bloodmoon poll failed", "error", err)
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.snap.Bloodmoon = bm
	p.snap.BloodmoonAt = p.now()
}

// safeError renders err for display to a logged-in operator, preferring the
// game server's own message and never including a stack trace.
func safeError(err error) string {
	var apiErr *sdtd.APIError
	if errors.As(err, &apiErr) {
		return apiErr.SafeMessage()
	}
	return err.Error()
}

package state

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nomansheikh/7dtd-panel/internal/sdtd"
)

// fakeClient is a scripted GameClient. Each call pops the next scripted
// outcome, so a test can describe an exact failure sequence.
type fakeClient struct {
	mu sync.Mutex

	statsErr  []error
	statsCall int
	stats     sdtd.ServerStats

	info    sdtd.ValueSet
	infoErr error

	logPage sdtd.LogPage
	logErr  error

	bloodmoon    sdtd.Bloodmoon
	bloodmoonErr error

	infoCalls      int
	logCalls       int
	bloodmoonCalls int
}

func (f *fakeClient) ServerStats(context.Context) (sdtd.ServerStats, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var err error
	if f.statsCall < len(f.statsErr) {
		err = f.statsErr[f.statsCall]
	}
	f.statsCall++
	if err != nil {
		return sdtd.ServerStats{}, err
	}
	return f.stats, nil
}

func (f *fakeClient) ServerInfo(context.Context) (sdtd.ValueSet, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.infoCalls++
	return f.info, f.infoErr
}

func (f *fakeClient) Log(context.Context, int, int) (sdtd.LogPage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.logCalls++
	return f.logPage, f.logErr
}

func (f *fakeClient) Bloodmoon(context.Context) (sdtd.Bloodmoon, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.bloodmoonCalls++
	return f.bloodmoon, f.bloodmoonErr
}

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newTestPoller(t *testing.T, client GameClient, threshold int) *Poller {
	t.Helper()
	return New(Options{
		Client:           client,
		Interval:         time.Second,
		FailureThreshold: threshold,
		Logger:           quietLogger(),
	})
}

func TestInitialStatusIsUnknown(t *testing.T) {
	p := newTestPoller(t, &fakeClient{}, 3)
	snap := p.Snapshot()
	if snap.Status != StatusUnknown {
		t.Errorf("status = %q, want unknown", snap.Status)
	}
	if snap.Reachable() {
		t.Error("an unpolled server should not be reachable")
	}
	if _, ok := snap.StatsAge(time.Now()); ok {
		t.Error("there should be no stats age before the first poll")
	}
}

func TestStatusBecomesOnlineAfterSuccess(t *testing.T) {
	client := &fakeClient{stats: sdtd.ServerStats{
		GameTime: sdtd.GameTime{Days: 3, Hours: 14, Minutes: 30},
		Players:  2,
	}}
	p := newTestPoller(t, client, 3)
	p.Tick(context.Background())

	snap := p.Snapshot()
	if snap.Status != StatusOnline {
		t.Errorf("status = %q, want online", snap.Status)
	}
	if snap.Stats.Players != 2 {
		t.Errorf("players = %d, want 2", snap.Stats.Players)
	}
	if snap.StatsAt.IsZero() {
		t.Error("StatsAt should be set after a successful poll")
	}
}

// TestSingleFailureDoesNotLoseStateOrGoOffline is the core requirement: a blip
// must not blank the UI or flip the badge to offline.
func TestSingleFailureDoesNotLoseStateOrGoOffline(t *testing.T) {
	client := &fakeClient{
		stats: sdtd.ServerStats{Players: 4, GameTime: sdtd.GameTime{Days: 9}},
		// First poll succeeds, second fails.
		statsErr: []error{nil, errors.New("connection refused")},
	}
	p := newTestPoller(t, client, 3)
	ctx := context.Background()

	p.Tick(ctx)
	good := p.Snapshot()

	p.Tick(ctx)
	after := p.Snapshot()

	if after.Status != StatusDegraded {
		t.Errorf("status = %q after one failure, want degraded", after.Status)
	}
	if after.Stats.Players != good.Stats.Players {
		t.Errorf("players went from %d to %d; last known good must be preserved",
			good.Stats.Players, after.Stats.Players)
	}
	if after.Stats.GameTime.Days != 9 {
		t.Errorf("game day was lost: %d", after.Stats.GameTime.Days)
	}
	if !after.StatsAt.Equal(good.StatsAt) {
		t.Error("StatsAt must not move on a failed poll, so the UI can show real staleness")
	}
	if after.LastError == "" {
		t.Error("the failure reason should be recorded")
	}
	if !after.Reachable() {
		t.Error("degraded still counts as reachable")
	}
}

func TestOfflineRequiresConsecutiveFailures(t *testing.T) {
	tests := []struct {
		name       string
		threshold  int
		failures   int
		wantStatus Status
	}{
		{"one failure of three", 3, 1, StatusDegraded},
		{"two failures of three", 3, 2, StatusDegraded},
		{"three failures of three", 3, 3, StatusOffline},
		{"four failures of three", 3, 4, StatusOffline},
		{"threshold of one goes offline immediately", 1, 1, StatusOffline},
		{"threshold of five tolerates four", 5, 4, StatusDegraded},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			script := []error{nil} // first poll succeeds
			for i := 0; i < tt.failures; i++ {
				script = append(script, errors.New("boom"))
			}
			client := &fakeClient{stats: sdtd.ServerStats{Players: 1}, statsErr: script}
			p := newTestPoller(t, client, tt.threshold)
			ctx := context.Background()

			for i := 0; i < tt.failures+1; i++ {
				p.Tick(ctx)
			}

			snap := p.Snapshot()
			if snap.Status != tt.wantStatus {
				t.Errorf("status = %q after %d failures (threshold %d), want %q",
					snap.Status, tt.failures, tt.threshold, tt.wantStatus)
			}
			if snap.ConsecutiveFailures != tt.failures {
				t.Errorf("consecutiveFailures = %d, want %d", snap.ConsecutiveFailures, tt.failures)
			}
			// Cached data survives even when offline, so the UI can show the
			// last thing it knew alongside a clear disconnected state.
			if snap.Stats.Players != 1 {
				t.Errorf("cached players = %d, want 1 retained", snap.Stats.Players)
			}
		})
	}
}

func TestFailingBeforeAnySuccessGoesStraightOffline(t *testing.T) {
	// With nothing cached there is no good state to protect, so reporting
	// degraded would imply data the panel does not have.
	client := &fakeClient{statsErr: []error{errors.New("no route to host")}}
	p := newTestPoller(t, client, 3)
	p.Tick(context.Background())

	snap := p.Snapshot()
	if snap.Status != StatusOffline {
		t.Errorf("status = %q, want offline", snap.Status)
	}
	if snap.Reachable() {
		t.Error("offline must not be reachable")
	}
}

func TestRecoveryResetsFailureCount(t *testing.T) {
	client := &fakeClient{
		stats:    sdtd.ServerStats{Players: 7},
		statsErr: []error{nil, errors.New("x"), errors.New("y"), errors.New("z"), nil},
	}
	p := newTestPoller(t, client, 3)
	ctx := context.Background()

	for i := 0; i < 4; i++ {
		p.Tick(ctx)
	}
	if got := p.Snapshot().Status; got != StatusOffline {
		t.Fatalf("status = %q, want offline before recovery", got)
	}

	p.Tick(ctx)
	snap := p.Snapshot()
	if snap.Status != StatusOnline {
		t.Errorf("status = %q, want online after recovery", snap.Status)
	}
	if snap.ConsecutiveFailures != 0 {
		t.Errorf("consecutiveFailures = %d, want 0", snap.ConsecutiveFailures)
	}
	if snap.LastError != "" {
		t.Errorf("lastError = %q, want cleared", snap.LastError)
	}
}

func TestSlowResourcesAreSkippedWhileOffline(t *testing.T) {
	// Hammering a server that is down achieves nothing and slows every tick by
	// the connection timeout.
	client := &fakeClient{statsErr: []error{errors.New("down"), errors.New("down")}}
	p := newTestPoller(t, client, 1)
	ctx := context.Background()

	p.Tick(ctx)
	p.Tick(ctx)

	if client.infoCalls != 0 || client.logCalls != 0 || client.bloodmoonCalls != 0 {
		t.Errorf("slow polls ran while offline: info=%d log=%d bloodmoon=%d",
			client.infoCalls, client.logCalls, client.bloodmoonCalls)
	}
}

func TestSlowResourcesAreFetchedOnceWhileOnline(t *testing.T) {
	client := &fakeClient{
		stats: sdtd.ServerStats{Players: 1},
		info: sdtd.NewValueSet([]sdtd.TypedValue{
			{Name: "ServerVersion", Type: "string", Raw: []byte(`"V.3.20.10"`)},
			{Name: "LevelName", Type: "string", Raw: []byte(`"Navezgane"`)},
			{Name: "MaxPlayers", Type: "int", Raw: []byte(`8`)},
			{Name: "CurrentPlayers", Type: "int", Raw: []byte(`1`)},
			{Name: "GameMode", Type: "string", Raw: []byte(`"Survival"`)},
		}),
		logPage: sdtd.LogPage{Entries: []sdtd.LogEntry{
			{ID: 10, Uptime: "90000", ISOTime: "2026-09-11T08:00:00.0000000+00:00"},
		}},
		bloodmoon: sdtd.Bloodmoon{Next: sdtd.GameTime{Days: 7, Hours: 22}},
	}
	p := newTestPoller(t, client, 3)
	ctx := context.Background()

	p.Tick(ctx)

	snap := p.Snapshot()
	if snap.Version != "V.3.20.10" {
		t.Errorf("version = %q", snap.Version)
	}
	if snap.World != "Navezgane" {
		t.Errorf("world = %q", snap.World)
	}
	if snap.MaxPlayers != 8 {
		t.Errorf("maxPlayers = %d, want 8", snap.MaxPlayers)
	}
	if snap.Bloodmoon.Next.Days != 7 {
		t.Errorf("next blood moon day = %d, want 7", snap.Bloodmoon.Next.Days)
	}
	if snap.Uptime != 90*time.Second {
		t.Errorf("uptime = %s, want 1m30s", snap.Uptime)
	}

	// A second immediate tick must not refetch the slow resources, since their
	// intervals have not elapsed.
	p.Tick(ctx)
	if client.infoCalls != 1 {
		t.Errorf("serverinfo fetched %d times in quick succession, want 1", client.infoCalls)
	}
	if client.bloodmoonCalls != 1 {
		t.Errorf("bloodmoon fetched %d times, want 1", client.bloodmoonCalls)
	}
}

func TestSlowResourceFailureDoesNotAffectStatus(t *testing.T) {
	// serverinfo is decoration. Losing it must not make the server look down
	// when serverstats is answering fine.
	client := &fakeClient{
		stats:        sdtd.ServerStats{Players: 3},
		infoErr:      errors.New("info exploded"),
		logErr:       errors.New("log exploded"),
		bloodmoonErr: errors.New("bloodmoon exploded"),
	}
	p := newTestPoller(t, client, 3)
	p.Tick(context.Background())

	snap := p.Snapshot()
	if snap.Status != StatusOnline {
		t.Errorf("status = %q, want online", snap.Status)
	}
	if snap.ConsecutiveFailures != 0 {
		t.Errorf("consecutiveFailures = %d, want 0", snap.ConsecutiveFailures)
	}
}

func TestUptimeExtrapolatesBetweenPolls(t *testing.T) {
	base := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	client := &fakeClient{
		stats: sdtd.ServerStats{},
		logPage: sdtd.LogPage{Entries: []sdtd.LogEntry{
			{ID: 1, Uptime: "60000"},
		}},
	}
	p := New(Options{
		Client:           client,
		Interval:         time.Second,
		FailureThreshold: 3,
		Logger:           quietLogger(),
		Now:              func() time.Time { return base },
	})
	p.Tick(context.Background())

	// Uptime ticks up with the wall clock rather than jumping once per poll.
	got, ok := p.Snapshot().UptimeAt(base.Add(30 * time.Second))
	if !ok {
		t.Fatal("UptimeAt reported no uptime")
	}
	if want := 90 * time.Second; got != want {
		t.Errorf("uptime = %s, want %s", got, want)
	}
}

func TestUptimeAbsentBeforeFirstSample(t *testing.T) {
	p := newTestPoller(t, &fakeClient{}, 3)
	if _, ok := p.Snapshot().UptimeAt(time.Now()); ok {
		t.Error("UptimeAt should report nothing before a sample")
	}
}

func TestUnparseableUptimeIsIgnored(t *testing.T) {
	client := &fakeClient{
		stats:   sdtd.ServerStats{},
		logPage: sdtd.LogPage{Entries: []sdtd.LogEntry{{ID: 1, Uptime: "not-a-number"}}},
	}
	p := newTestPoller(t, client, 3)
	p.Tick(context.Background())

	if _, ok := p.Snapshot().UptimeAt(time.Now()); ok {
		t.Error("an unparseable uptime should leave uptime unset rather than zero")
	}
	if p.Snapshot().Status != StatusOnline {
		t.Error("a bad uptime must not affect status")
	}
}

func TestLastErrorUsesTheServersOwnMessageWithoutTrace(t *testing.T) {
	client := &fakeClient{statsErr: []error{&sdtd.APIError{
		Status:           http.StatusForbidden,
		ErrorCode:        "Unsupported",
		ExceptionMessage: "permission denied",
		Trace:            "at Webserver.Foo()",
	}}}
	p := newTestPoller(t, client, 1)
	p.Tick(context.Background())

	last := p.Snapshot().LastError
	if last == "" {
		t.Fatal("LastError is empty")
	}
	if want := "Unsupported: permission denied"; last != want {
		t.Errorf("lastError = %q, want %q", last, want)
	}
	// Traces belong in the server log, never in something the UI renders.
	if strings.Contains(last, "at Webserver.Foo()") {
		t.Error("LastError leaks the stack trace")
	}
}

func TestRunStopsOnContextCancel(t *testing.T) {
	client := &fakeClient{stats: sdtd.ServerStats{}}
	p := newTestPoller(t, client, 3)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		p.Run(ctx)
		close(done)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after the context was cancelled")
	}
}

func TestSnapshotIsSafeForConcurrentReads(t *testing.T) {
	client := &fakeClient{stats: sdtd.ServerStats{Players: 1}}
	p := newTestPoller(t, client, 3)
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_ = p.Snapshot()
			}
		}()
	}
	for i := 0; i < 50; i++ {
		p.Tick(ctx)
	}
	wg.Wait()
}

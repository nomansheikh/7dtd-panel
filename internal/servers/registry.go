// Package servers holds the per-server runtime.
//
// Everything that talks to a game server — the client, the poller, the log
// stream, the event hub and the catalogues — exists once per configured
// server. Nothing is shared between them: two servers have different worlds,
// different logs and different credentials, and mixing any of it would show an
// operator one server's data while they believed they were looking at another.
package servers

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/nomansheikh/7dtd-panel/internal/catalog"
	"github.com/nomansheikh/7dtd-panel/internal/config"
	"github.com/nomansheikh/7dtd-panel/internal/events"
	"github.com/nomansheikh/7dtd-panel/internal/sdtd"
	"github.com/nomansheikh/7dtd-panel/internal/state"
)

// The collaborators are interfaces so a test can assemble a Server from fakes
// without standing up a real client, poller and log stream for each one.

// Executor runs console commands and reads the player list.
//
// Console commands are the only write path for game state. Players is here
// rather than on the poller because it merges two endpoints and is read on
// demand, not on a schedule.
type Executor interface {
	Execute(ctx context.Context, command string) (sdtd.CommandResult, error)
	Players(ctx context.Context) ([]sdtd.Player, error)
	GamePrefs(ctx context.Context) (sdtd.ValueSet, error)
	// ReadGamePref reads a preference's live value through the console,
	// because /api/gameprefs does not reflect runtime changes.
	ReadGamePref(ctx context.Context, name string) (string, error)
	// GamePrefsLive does the same for every preference at once.
	GamePrefsLive(ctx context.Context) (map[string]string, error)
	// SandboxSettings supplies the descriptions and allowed values the
	// settings page needs to render pickers instead of text fields.
	SandboxSettings(ctx context.Context) (sdtd.SandboxSettings, error)
	// CurrentWeather reads what the weather is doing, which has no REST
	// endpoint and so comes back through the console.
	CurrentWeather(ctx context.Context) (sdtd.Weather, error)

	// ServerHealth reads load and version figures, which likewise have no REST
	// endpoint of their own.
	ServerHealth(ctx context.Context) (sdtd.Health, error)

	// ItemIcon fetches one item's art, which the panel proxies so the browser
	// never has to reach the game server itself.
	ItemIcon(ctx context.Context, name, tint string) ([]byte, error)
}

// Snapshotter supplies the cached view of a server.
type Snapshotter interface {
	Snapshot() state.Snapshot
}

// CommandCatalogue supplies the server's own command list.
type CommandCatalogue interface {
	Get(ctx context.Context) ([]sdtd.Command, time.Time, error)
}

// ItemCatalogue and EntityCatalogue back the give and spawn pickers.
type ItemCatalogue interface {
	Search(ctx context.Context, query string, includeBlocks bool, limit int) ([]sdtd.Item, int, error)
	Has(ctx context.Context, name string) (bool, error)
}

type EntityCatalogue interface {
	Search(ctx context.Context, query string, spawnableOnly bool, limit int) ([]sdtd.EntityClass, int, error)
	Lookup(ctx context.Context, name string) (sdtd.EntityClass, bool, error)
}

// BuffCatalogue backs the buff picker and rejects names the server would not
// recognise.
type BuffCatalogue interface {
	Search(ctx context.Context, query string, limit int) ([]sdtd.Buff, int, error)
	Has(ctx context.Context, name string) (bool, error)
}

// EventFeed is the hub browser clients subscribe to.
type EventFeed interface {
	Subscribe(backlog int) ([]events.Event, <-chan events.Event, func())
}

// Server is one game server and everything the panel keeps for it.
type Server struct {
	ID   string
	Name string
	// BaseURL is exposed for diagnostics. It contains no credentials.
	BaseURL string

	Client   Executor
	Poller   Snapshotter
	Events   EventFeed
	Commands CommandCatalogue
	Items    ItemCatalogue
	Entities EntityCatalogue
	Buffs    BuffCatalogue

	// These are the concrete collaborators Run needs. A Server assembled by a
	// test leaves them nil and is simply never Run.
	runner  *state.Poller
	stream  *sdtd.LogStreamer
	publish func(sdtd.LogEntry)
	warm    func(context.Context)
}

// Registry holds every configured server, in the order they were configured.
type Registry struct {
	ordered []*Server
	byID    map[string]*Server
}

// New builds the runtime for every configured server.
//
// No network calls happen here: the panel must start and serve a clear
// disconnected state even when every game server is down.
func New(cfg config.Config, log *slog.Logger, pollInterval time.Duration, failureThreshold int) (*Registry, error) {
	r := &Registry{byID: make(map[string]*Server, len(cfg.Servers))}

	for _, gc := range cfg.Servers {
		serverLog := log.With("server", gc.ID)

		client, err := sdtd.New(sdtd.Options{
			BaseURL:     gc.BaseURL(),
			TokenName:   gc.TokenName,
			TokenSecret: gc.TokenSecret,
			Timeout:     10 * time.Second,
		})
		if err != nil {
			return nil, fmt.Errorf("server %s: %w", gc.ID, err)
		}

		hub := events.NewHub()
		items := catalog.NewItems(client, time.Hour, serverLog.With("component", "catalog"))
		srv := &Server{
			ID:       gc.ID,
			Name:     gc.Name,
			BaseURL:  gc.BaseURL(),
			Client:   client,
			Events:   hub,
			Commands: catalog.NewCommands(client, time.Hour, serverLog.With("component", "catalog")),
			Items:    items,
			Entities: catalog.NewEntities(client, time.Hour, serverLog.With("component", "catalog")),
			Buffs:    catalog.NewBuffs(client, time.Hour, serverLog.With("component", "catalog")),
			stream:   sdtd.NewLogStreamer(client, serverLog.With("component", "logstream")),
			publish:  hub.PublishLog,
			warm:     items.Warm,
		}

		poller := state.New(state.Options{
			Client:           client,
			Interval:         pollInterval,
			FailureThreshold: failureThreshold,
			Logger:           serverLog.With("component", "poller"),
			OnStatusChange: func(from, to state.Status) {
				hub.PublishStatus("Game server " + string(to) + " (was " + string(from) + ")")
			},
		})
		srv.Poller = poller
		srv.runner = poller

		r.ordered = append(r.ordered, srv)
		r.byID[srv.ID] = srv
	}

	if len(r.ordered) == 0 {
		return nil, fmt.Errorf("no game servers configured")
	}
	return r, nil
}

// NewRegistry assembles a registry from already-built servers, in the order
// given. It exists for tests, and for any future source of servers that is not
// the environment.
//
// Servers built this way have no poller or log stream of their own, so Run must
// not be called on the result.
func NewRegistry(list ...*Server) *Registry {
	r := &Registry{byID: make(map[string]*Server, len(list))}
	for _, srv := range list {
		r.ordered = append(r.ordered, srv)
		r.byID[srv.ID] = srv
	}
	return r
}

// All returns every server in configuration order.
func (r *Registry) All() []*Server { return r.ordered }

// Get finds a server by ID.
func (r *Registry) Get(id string) (*Server, bool) {
	s, ok := r.byID[id]
	return s, ok
}

// Default is the server the UI opens on: the first configured.
func (r *Registry) Default() *Server { return r.ordered[0] }

// Run starts polling and streaming for every server, and blocks until ctx is
// cancelled.
//
// One unreachable server must not delay or stop the others, so each runs in its
// own goroutines and nothing here waits on a network call.
func (r *Registry) Run(ctx context.Context) {
	var wg sync.WaitGroup

	for _, srv := range r.ordered {
		wg.Add(3)
		go func() {
			defer wg.Done()
			srv.runner.Run(ctx)
		}()
		go func() {
			defer wg.Done()
			srv.stream.Run(ctx, srv.publish)
		}()
		go func() {
			defer wg.Done()
			// The item catalogue is ~2.9 MB per server, so it is warmed in the
			// background rather than making the first picker keystroke wait.
			srv.warm(ctx)
			<-ctx.Done()
		}()
	}

	wg.Wait()
}

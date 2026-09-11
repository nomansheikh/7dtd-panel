// Package api serves the panel's own HTTP API: the only thing the browser
// talks to.
//
// There is deliberately a handler per feature rather than a pass-through proxy
// to the game server. A catch-all would expose /api/webapitokens, which returns
// token secrets in plaintext, and /api/webusers, neither of which the panel has
// any business surfacing.
package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/nomansheikh/7dtd-panel/internal/auth"
	"github.com/nomansheikh/7dtd-panel/internal/config"
	"github.com/nomansheikh/7dtd-panel/internal/events"
	"github.com/nomansheikh/7dtd-panel/internal/sdtd"
	"github.com/nomansheikh/7dtd-panel/internal/state"
	"github.com/nomansheikh/7dtd-panel/internal/store"
)

// Snapshotter supplies the cached game server view.
type Snapshotter interface {
	Snapshot() state.Snapshot
}

// GameExecutor is the slice of the game client the API uses directly. The
// poller owns everything read on a schedule; this is for operator-initiated
// actions.
type GameExecutor interface {
	Execute(ctx context.Context, command string) (sdtd.CommandResult, error)
}

// CommandCatalogue supplies the server's command list, cached.
type CommandCatalogue interface {
	Get(ctx context.Context) ([]sdtd.Command, time.Time, error)
}

// ItemCatalogue and EntityCatalogue back the give and spawn pickers. Search
// happens here rather than in the browser because the item list is ~2.9 MB.
type ItemCatalogue interface {
	Search(ctx context.Context, query string, includeBlocks bool, limit int) ([]sdtd.Item, int, error)
	Has(ctx context.Context, name string) (bool, error)
}

type EntityCatalogue interface {
	Search(ctx context.Context, query string, spawnableOnly bool, limit int) ([]sdtd.EntityClass, int, error)
	Lookup(ctx context.Context, name string) (sdtd.EntityClass, bool, error)
}

// EventFeed is the hub browser clients subscribe to.
type EventFeed interface {
	Subscribe(backlog int) ([]events.Event, <-chan events.Event, func())
}

// Deps are the collaborators a Server needs.
type Deps struct {
	Config   config.Config
	Store    *store.Store
	Sessions *auth.Sessions
	State    Snapshotter
	Game     GameExecutor
	Commands CommandCatalogue
	Items    ItemCatalogue
	Entities EntityCatalogue
	Events   EventFeed
	Logger   *slog.Logger
	Version  string
	Now      func() time.Time
}

// Server holds the panel's API handlers.
type Server struct {
	cfg      config.Config
	store    *store.Store
	sessions *auth.Sessions
	state    Snapshotter
	game     GameExecutor
	commands CommandCatalogue
	items    ItemCatalogue
	entities EntityCatalogue
	events   EventFeed
	log      *slog.Logger
	version  string
	now      func() time.Time

	loginLimiter *rateLimiter
}

// NewServer builds the API.
func NewServer(d Deps) *Server {
	now := d.Now
	if now == nil {
		now = time.Now
	}
	log := d.Logger
	if log == nil {
		log = slog.Default()
	}
	return &Server{
		cfg:      d.Config,
		store:    d.Store,
		sessions: d.Sessions,
		state:    d.State,
		game:     d.Game,
		commands: d.Commands,
		items:    d.Items,
		entities: d.Entities,
		events:   d.Events,
		log:      log,
		version:  d.Version,
		now:      now,
		// Ten attempts a minute per address is generous for a human and
		// useless for a password guesser.
		loginLimiter: newRateLimiter(10, time.Minute),
	}
}

// Routes returns the API mux. Patterns use Go 1.22 method+path matching, so no
// router dependency is needed.
func (s *Server) Routes() *http.ServeMux {
	mux := http.NewServeMux()

	// Unauthenticated. Health must stay open so container orchestration can
	// probe it without credentials.
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	mux.HandleFunc("POST /api/auth/logout", s.handleLogout)

	// Authenticated.
	mux.Handle("GET /api/auth/me", s.requireAuth(http.HandlerFunc(s.handleMe)))
	mux.Handle("GET /api/dashboard", s.requireAuth(http.HandlerFunc(s.handleDashboard)))
	mux.Handle("GET /api/events", s.requireAuth(http.HandlerFunc(s.handleEvents)))
	mux.Handle("GET /api/console/commands", s.requireAuth(http.HandlerFunc(s.handleConsoleCommands)))
	mux.Handle("POST /api/console/execute", s.requireAuth(http.HandlerFunc(s.handleConsoleExecute)))
	mux.Handle("GET /api/console/history", s.requireAuth(http.HandlerFunc(s.handleConsoleHistory)))

	// World controls. Every one of these becomes a console command, because
	// the REST API has no write path for game state.
	mux.Handle("POST /api/world/time", s.requireAuth(http.HandlerFunc(s.handleSetTime)))
	mux.Handle("POST /api/world/weather", s.requireAuth(http.HandlerFunc(s.handleWeather)))
	mux.Handle("POST /api/world/spawn", s.requireAuth(http.HandlerFunc(s.handleSpawn)))
	mux.Handle("POST /api/world/horde", s.requireAuth(http.HandlerFunc(s.handleWanderingHorde)))
	mux.Handle("POST /api/world/say", s.requireAuth(http.HandlerFunc(s.handleSay)))

	mux.Handle("GET /api/items", s.requireAuth(http.HandlerFunc(s.handleItems)))
	mux.Handle("GET /api/entities", s.requireAuth(http.HandlerFunc(s.handleEntities)))

	return mux
}

// CleanupSessions removes expired sessions until ctx is cancelled.
func (s *Server) CleanupSessions(ctx context.Context, every time.Duration) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n, err := s.sessions.Cleanup(ctx)
			if err != nil {
				s.log.Warn("session cleanup failed", "error", err)
				continue
			}
			if n > 0 {
				s.log.Debug("expired sessions removed", "count", n)
			}
		}
	}
}

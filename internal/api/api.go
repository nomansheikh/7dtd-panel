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
	"github.com/nomansheikh/7dtd-panel/internal/servers"
	"github.com/nomansheikh/7dtd-panel/internal/store"
)

// Deps are the collaborators a Server needs.
type Deps struct {
	Config   config.Config
	Store    *store.Store
	Sessions *auth.Sessions
	Servers  *servers.Registry
	Logger   *slog.Logger
	Version  string
	Now      func() time.Time
}

// Server holds the panel's API handlers.
type Server struct {
	cfg      config.Config
	store    *store.Store
	sessions *auth.Sessions
	registry *servers.Registry
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
		registry: d.Servers,
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

	mux.Handle("GET /api/auth/me", s.requireAuth(http.HandlerFunc(s.handleMe)))

	// The list of servers is not server-scoped: it is how the UI finds out
	// which servers exist in the first place.
	mux.Handle("GET /api/servers", s.requireAuth(http.HandlerFunc(s.handleServers)))

	// Everything below acts on one game server, named in the path. Mixing two
	// servers' data would show an operator one world while they believed they
	// were looking at another, so the server is resolved once, centrally.
	scoped := func(pattern string, h http.HandlerFunc) {
		mux.Handle(pattern, s.requireAuth(s.withServer(h)))
	}

	scoped("GET /api/servers/{server}/dashboard", s.handleDashboard)
	scoped("GET /api/servers/{server}/players", s.handlePlayers)

	// Player actions. Each names its target by entity id, because a display
	// name is chosen by the player and an integer cannot carry a payload.
	scoped("POST /api/servers/{server}/players/{entityId}/teleport", s.handleTeleport)
	scoped("POST /api/servers/{server}/players/{entityId}/give", s.handleGiveItem)
	scoped("POST /api/servers/{server}/players/{entityId}/kill", s.handleKillPlayer)
	scoped("POST /api/servers/{server}/players/{entityId}/kick", s.handleKick)
	scoped("POST /api/servers/{server}/players/{entityId}/buff", s.handleBuff)
	scoped("POST /api/servers/{server}/players/{entityId}/xp", s.handleGiveXP)
	// Ban takes a platform id, not an entity id: it is the one action that has
	// to work for someone who has already left.
	scoped("POST /api/servers/{server}/players/ban", s.handleBan)
	scoped("POST /api/servers/{server}/players/unban", s.handleUnban)
	scoped("GET /api/servers/{server}/events", s.handleEvents)

	scoped("GET /api/servers/{server}/console/commands", s.handleConsoleCommands)
	scoped("POST /api/servers/{server}/console/execute", s.handleConsoleExecute)
	scoped("GET /api/servers/{server}/console/history", s.handleConsoleHistory)

	scoped("POST /api/servers/{server}/world/time", s.handleSetTime)
	scoped("POST /api/servers/{server}/world/weather", s.handleWeather)
	scoped("POST /api/servers/{server}/world/spawn", s.handleSpawn)
	scoped("POST /api/servers/{server}/world/horde", s.handleWanderingHorde)
	scoped("POST /api/servers/{server}/world/say", s.handleSay)

	scoped("GET /api/servers/{server}/settings", s.handleSettings)
	scoped("PUT /api/servers/{server}/settings/{name}", s.handleUpdateSetting)

	scoped("GET /api/servers/{server}/items", s.handleItems)
	scoped("GET /api/servers/{server}/entities", s.handleEntities)
	scoped("GET /api/servers/{server}/buffs", s.handleBuffs)

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

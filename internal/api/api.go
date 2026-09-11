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
	"github.com/nomansheikh/7dtd-panel/internal/state"
	"github.com/nomansheikh/7dtd-panel/internal/store"
)

// Snapshotter supplies the cached game server view.
type Snapshotter interface {
	Snapshot() state.Snapshot
}

// Deps are the collaborators a Server needs.
type Deps struct {
	Config   config.Config
	Store    *store.Store
	Sessions *auth.Sessions
	State    Snapshotter
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

package servers

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

/*
The registry, which can now change while the panel is running.

It used to be built once at startup from environment variables and never touch
it again, which made adding a server mean editing a compose file and restarting
the container. Servers are added from the panel now, so everything a server owns
— its client, poller, log stream, event hub, catalogues, chat bot and scheduler
— has to be able to start and stop on its own.

Each server runs under a context derived from the one Run was given, so removing
one stops exactly its goroutines and leaves the rest alone.
*/

// Attach starts whatever else belongs to a server: the chat bot and the task
// runner. It is a callback so this package does not have to import them, which
// would be a cycle in spirit if not in fact — those two are built on top of a
// server, not part of it.
type Attach func(ctx context.Context, srv *Server)

// Registry holds every configured server.
type Registry struct {
	log              *slog.Logger
	pollInterval     time.Duration
	failureThreshold int
	attach           Attach

	mu      sync.RWMutex
	ordered []*Server
	byID    map[string]*Server
	stop    map[string]context.CancelFunc

	// running is the context Run was given, held so a server added later can
	// be started under it. Nil until Run is called, which is the case in tests.
	running context.Context
	wg      sync.WaitGroup
}

// Options configures a Registry.
type Options struct {
	Logger           *slog.Logger
	PollInterval     time.Duration
	FailureThreshold int
	// Attach is optional; a registry without one still polls and streams.
	Attach Attach
}

// New builds an empty registry. Servers are added with Add.
func New(opts Options) *Registry {
	if opts.Logger == nil {
		opts.Logger = slog.New(slog.DiscardHandler)
	}
	return &Registry{
		log:              opts.Logger,
		pollInterval:     opts.PollInterval,
		failureThreshold: opts.FailureThreshold,
		attach:           opts.Attach,
		byID:             make(map[string]*Server),
		stop:             make(map[string]context.CancelFunc),
	}
}

// NewRegistry assembles a registry from already-built servers, in the order
// given. It exists for tests, and for any future source of servers that is not
// the database.
//
// Servers built this way have no poller or log stream of their own, so they are
// never started.
func NewRegistry(list ...*Server) *Registry {
	r := New(Options{})
	for _, srv := range list {
		r.ordered = append(r.ordered, srv)
		r.byID[srv.ID] = srv
	}
	return r
}

// All returns every server, in display order.
func (r *Registry) All() []*Server {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Server, len(r.ordered))
	copy(out, r.ordered)
	return out
}

// Get finds a server by ID.
func (r *Registry) Get(id string) (*Server, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.byID[id]
	return s, ok
}

// Default is the server the UI opens on: the first in display order. Nil when
// none are configured, which is now a state the panel starts in.
func (r *Registry) Default() *Server {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.ordered) == 0 {
		return nil
	}
	return r.ordered[0]
}

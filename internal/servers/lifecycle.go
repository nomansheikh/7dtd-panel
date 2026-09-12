package servers

import (
	"context"
	"sort"

	"github.com/nomansheikh/7dtd-panel/internal/config"
)

// Servers coming and going while the panel runs. Each one lives under a context
// derived from the one Run was given, so removing one stops exactly its
// goroutines and leaves the rest alone.

/*
Add builds a server's runtime and starts it.

Replacing an existing id stops the old one first, so editing a server's host or
token takes effect without a restart — which is the entire point of moving this
out of the environment.
*/
func (r *Registry) Add(gc config.Game) (*Server, error) {
	srv, err := build(gc, r.log, r.pollInterval, r.failureThreshold)
	if err != nil {
		return nil, err
	}

	r.mu.Lock()
	if _, exists := r.byID[gc.ID]; exists {
		r.mu.Unlock()
		r.Remove(gc.ID)
		r.mu.Lock()
	}
	r.byID[gc.ID] = srv
	r.ordered = append(r.ordered, srv)
	parent := r.running
	r.mu.Unlock()

	if parent != nil {
		r.start(parent, srv)
	}
	return srv, nil
}

// Remove stops a server and forgets it.
func (r *Registry) Remove(id string) bool {
	r.mu.Lock()
	srv, ok := r.byID[id]
	if !ok {
		r.mu.Unlock()
		return false
	}
	cancel := r.stop[id]
	delete(r.byID, id)
	delete(r.stop, id)
	for i, s := range r.ordered {
		if s.ID == id {
			r.ordered = append(r.ordered[:i], r.ordered[i+1:]...)
			break
		}
	}
	r.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	r.log.Info("game server removed", "server", srv.ID)
	return true
}

// Reorder puts the servers in the given order, ignoring ids it does not know.
func (r *Registry) Reorder(ids []string) {
	rank := make(map[string]int, len(ids))
	for i, id := range ids {
		rank[id] = i
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	sort.SliceStable(r.ordered, func(i, j int) bool {
		a, aok := rank[r.ordered[i].ID]
		b, bok := rank[r.ordered[j].ID]
		if aok && bok {
			return a < b
		}
		return aok && !bok
	})
}

/*
Run starts every server known so far and keeps the registry running until ctx
is cancelled.

It returns when the context is done rather than when the servers finish, since
servers can come and go while it runs.
*/
func (r *Registry) Run(ctx context.Context) {
	r.mu.Lock()
	r.running = ctx
	known := make([]*Server, len(r.ordered))
	copy(known, r.ordered)
	r.mu.Unlock()

	for _, srv := range known {
		r.start(ctx, srv)
	}

	<-ctx.Done()
	r.wg.Wait()
}

// start runs one server's goroutines under its own cancellable context.
func (r *Registry) start(parent context.Context, srv *Server) {
	if srv.runner == nil {
		// Assembled by a test from fakes; there is nothing to run.
		return
	}
	ctx, cancel := context.WithCancel(parent)

	r.mu.Lock()
	r.stop[srv.ID] = cancel
	r.mu.Unlock()

	r.wg.Add(3)
	go func() {
		defer r.wg.Done()
		srv.runner.Run(ctx)
	}()
	go func() {
		defer r.wg.Done()
		srv.stream.Run(ctx, srv.publish)
	}()
	go func() {
		defer r.wg.Done()
		// The item catalogue is ~2.9 MB per server, so it is warmed in the
		// background rather than making the first picker keystroke wait.
		srv.warm(ctx)
		<-ctx.Done()
	}()

	if r.attach != nil {
		r.attach(ctx, srv)
	}
	r.log.Info("game server started", "server", srv.ID, "url", srv.BaseURL)
}

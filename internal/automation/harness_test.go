package automation

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/nomansheikh/7dtd-panel/internal/events"
	"github.com/nomansheikh/7dtd-panel/internal/sdtd"
	"github.com/nomansheikh/7dtd-panel/internal/state"
	"github.com/nomansheikh/7dtd-panel/internal/store"
)

// The fakes every test in this package runs against: a client that records what
// it was asked to send, a poller that answers with whatever world the test set
// up, and a real store, since the bookkeeping is half of what is being tested.

type fakeClient struct {
	mu   sync.Mutex
	sent []string
	err  error
}

func (c *fakeClient) Execute(_ context.Context, command string) (sdtd.CommandResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sent = append(c.sent, command)
	return sdtd.CommandResult{}, c.err
}

func (c *fakeClient) commands() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, len(c.sent))
	copy(out, c.sent)
	return out
}

type fakePoller struct{ snap state.Snapshot }

func (p fakePoller) Snapshot() state.Snapshot { return p.snap }

type rig struct {
	runner *Runner
	client *fakeClient
	db     *store.Store
	hub    *events.Hub
	now    time.Time
}

func newRig(t *testing.T) *rig {
	t.Helper()
	db, err := store.Open(t.Context(), t.TempDir()+"/panel.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	r := &rig{
		client: &fakeClient{},
		db:     db,
		hub:    events.NewHub(),
		now:    time.Date(2026, 9, 12, 4, 0, 0, 0, time.Local),
	}
	r.runner = New(Options{
		Server:           "test",
		Feed:             r.hub,
		Client:           r.client,
		Poller:           fakePoller{snap: snap(7, 4, 7, 60)},
		Store:            db,
		Now:              func() time.Time { return r.now },
		AllowDestructive: true,
	})
	return r
}

func (r *rig) save(t *testing.T, task store.Task) {
	t.Helper()
	task.Enabled = true
	if err := r.db.SaveTask(t.Context(), "test", task, r.now); err != nil {
		t.Fatal(err)
	}
}

func (r *rig) runs(t *testing.T) []store.TaskRun {
	t.Helper()
	got, err := r.db.RecentTaskRuns(t.Context(), "test", 50)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

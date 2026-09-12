package power

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/nomansheikh/7dtd-panel/internal/sdtd"
	"github.com/nomansheikh/7dtd-panel/internal/state"
)

/*
The clock is injected, so a fifteen-minute countdown runs in microseconds and
the tests assert on what was said and when rather than on having waited.
*/

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

// world reports whatever status the test sets, so a server can be made to go
// away and come back.
type world struct {
	mu     sync.Mutex
	status state.Status
}

func (w *world) Snapshot() state.Snapshot {
	w.mu.Lock()
	defer w.mu.Unlock()
	return state.Snapshot{Status: w.status}
}

func (w *world) becomes(s state.Status) {
	w.mu.Lock()
	w.status = s
	w.mu.Unlock()
}

type rig struct {
	c      *Controller
	client *fakeClient
	world  *world

	mu  sync.Mutex
	now time.Time
	// ticks counts how many times the sequence has waited. onTick lets a test
	// change the world at an exact point, which beats racing a goroutine
	// against a clock that advances in microseconds.
	ticks  int
	onTick func(tick int)
}

func newRig(t *testing.T) *rig {
	t.Helper()
	r := &rig{
		client: &fakeClient{},
		world:  &world{status: state.StatusOnline},
		now:    time.Unix(1_700_000_000, 0).UTC(),
	}
	r.c = New(Options{
		Server:           "test",
		Client:           r.client,
		Poller:           r.world,
		AllowDestructive: true,
		Now:              r.clock,
		// Time passes when the sequence asks for it to, not when the wall
		// clock says so.
		Sleep: func(ctx context.Context, d time.Duration) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			// The real sleep returns at once for a non-positive duration
			// without waiting, so neither does this — otherwise a countdown of
			// zero silently consumes a tick the test was counting on.
			if d <= 0 {
				return nil
			}
			r.mu.Lock()
			r.now = r.now.Add(d)
			r.ticks++
			tick, hook := r.ticks, r.onTick
			r.mu.Unlock()
			if hook != nil {
				hook(tick)
			}
			return nil
		},
	})
	return r
}

func (r *rig) clock() time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.now
}

// settle waits for the sequence goroutine to reach a resting phase.
func (r *rig) settle(t *testing.T, want ...Phase) Status {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		s := r.c.Status()
		for _, w := range want {
			if s.Phase == w {
				return s
			}
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("never reached %v; stuck at %q", want, r.c.Status().Phase)
	return Status{}
}

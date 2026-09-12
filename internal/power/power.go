/*
Package power stops a game server, and watches to see whether it comes back.

The panel can stop a server and can never start one: it talks to the server
over that server's own HTTP API, so once the process is gone there is nothing
left to talk to. Whether a stop is a restart depends entirely on something
outside the panel — a systemd unit, a Docker restart policy, a game-server
manager. That is worth being honest about in the interface rather than
labelling a button "restart" and hoping.

What the panel can do, and a person at a console cannot easily, is run the
whole sequence unattended: warn at intervals, save, stop, then keep watching
and say whether the thing came back and how long it took. The last part is the
only answer anybody actually wants at five in the morning.
*/
package power

import (
	"context"
	"sync"
	"time"

	"github.com/nomansheikh/7dtd-panel/internal/sdtd"
	"github.com/nomansheikh/7dtd-panel/internal/state"
)

// Phase is where a sequence has got to.
type Phase string

const (
	// PhaseIdle is nothing happening, which is almost always.
	PhaseIdle Phase = "idle"
	// PhaseCountdown is warning players at intervals.
	PhaseCountdown Phase = "countdown"
	// PhaseSaving is the saveworld before the lights go out.
	PhaseSaving Phase = "saving"
	// PhaseStopping is the shutdown command going in.
	PhaseStopping Phase = "stopping"
	// PhaseWatching is after the stop, waiting to see if it returns.
	PhaseWatching Phase = "watching"
	// PhaseBack is it came back, and how long it took.
	PhaseBack Phase = "back"
	// PhaseGone is it stopped and did not return. For a stop that is success;
	// for a restart it means nothing is supervising the process.
	PhaseGone Phase = "gone"
	// PhaseFailed is the sequence itself went wrong.
	PhaseFailed Phase = "failed"
	// PhaseCancelled is somebody called it off before the stop.
	PhaseCancelled Phase = "cancelled"
)

// Intent is what the operator asked for, which decides how the outcome reads.
type Intent string

const (
	// IntentRestart expects something else to bring the server back.
	IntentRestart Intent = "restart"
	// IntentStop does not.
	IntentStop Intent = "stop"
)

// Status is what the panel knows about the sequence right now.
type Status struct {
	Phase  Phase  `json:"phase"`
	Intent Intent `json:"intent,omitempty"`

	// StartedAt is when the sequence began, StopAt when the server will be
	// told to stop. Both absent when nothing is happening.
	StartedAt time.Time `json:"startedAt,omitzero"`
	StopAt    time.Time `json:"stopAt,omitzero"`

	// DownSeconds is how long it was away, once it is back.
	DownSeconds int `json:"downSeconds,omitempty"`
	// Problem is why it failed, already safe to show an operator.
	Problem string `json:"problem,omitempty"`
	// Note carries the finding when a restart did not come back.
	Note string `json:"note,omitempty"`
}

// Executor runs console commands.
type Executor interface {
	Execute(ctx context.Context, command string) (sdtd.CommandResult, error)
}

// Snapshotter is how the panel sees whether the server is answering.
type Snapshotter interface {
	Snapshot() state.Snapshot
}

// Announcer puts a line in the panel's own feed.
type Announcer interface {
	PublishStatus(message string)
}

// Controller runs one server's power sequence. Only one at a time.
type Controller struct {
	opts Options

	mu     sync.Mutex
	status Status
	cancel context.CancelFunc
}

// Options configures a Controller.
type Options struct {
	Server   string
	Client   Executor
	Poller   Snapshotter
	Announce Announcer
	Now      func() time.Time
	// Sleep exists so a test can run a fifteen-minute countdown instantly.
	Sleep func(ctx context.Context, d time.Duration) error
	// AllowDestructive mirrors PANEL_ALLOW_DESTRUCTIVE. A stop is a stop
	// however it is dressed up, so the operator's own switch applies.
	AllowDestructive bool
}

// New builds a Controller. It contacts nothing.
func New(opts Options) *Controller {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Sleep == nil {
		opts.Sleep = sleep
	}
	return &Controller{opts: opts, status: Status{Phase: PhaseIdle}}
}

// Status returns what is happening, safe to call at any time.
func (c *Controller) Status() Status {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.status
}

func (c *Controller) set(mutate func(*Status)) {
	c.mu.Lock()
	mutate(&c.status)
	c.mu.Unlock()
}

// sleep waits, or gives up early when the sequence is cancelled.
func sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

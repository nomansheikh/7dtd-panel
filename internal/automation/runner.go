package automation

import (
	"context"
	"log/slog"
	"time"

	"github.com/nomansheikh/7dtd-panel/internal/events"
	"github.com/nomansheikh/7dtd-panel/internal/sdtd"
	"github.com/nomansheikh/7dtd-panel/internal/state"
	"github.com/nomansheikh/7dtd-panel/internal/store"
)

// The collaborators are interfaces so a test can run a whole schedule without a
// game server, a poller or a clock that actually advances.

// Executor runs console commands.
type Executor interface {
	Execute(ctx context.Context, command string) (sdtd.CommandResult, error)
}

// Snapshotter supplies the cached view of the world, which is where the game
// clock and the blood moon come from.
type Snapshotter interface {
	Snapshot() state.Snapshot
}

// Feed is the event hub, for the triggers that are not about time.
type Feed interface {
	Subscribe(backlog int) ([]events.Event, <-chan events.Event, func())
	PublishStatus(message string)
}

// Store holds the tasks and the record of what they did.
type Store interface {
	Tasks(ctx context.Context, serverID string) ([]store.Task, error)
	ClaimTask(ctx context.Context, serverID, name string, expect store.Task, newKey string, now time.Time) (bool, error)
	SetTaskKey(ctx context.Context, serverID, name string, expect store.Task, newKey string) error
	AddTaskRun(ctx context.Context, serverID string, run store.TaskRun) error
}

// Options configures a Runner.
type Options struct {
	Server   string
	Feed     Feed
	Client   Executor
	Poller   Snapshotter
	Store    Store
	Logger   *slog.Logger
	Now      func() time.Time
	Interval time.Duration
	// AllowDestructive mirrors PANEL_ALLOW_DESTRUCTIVE, honoured here as it is
	// everywhere else: a schedule is not a way around a setting the operator
	// chose.
	AllowDestructive bool
}

// Runner carries out one server's tasks.
type Runner struct {
	opts Options
	now  func() time.Time
	log  *slog.Logger
}

// tick is how often the clock-driven triggers are checked. Half a minute is
// finer than any of them need and costs a read of a small table.
const tick = 30 * time.Second

// New builds a Runner. It contacts nothing.
func New(opts Options) *Runner {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Logger == nil {
		opts.Logger = slog.New(slog.DiscardHandler)
	}
	if opts.Interval <= 0 {
		opts.Interval = tick
	}
	return &Runner{opts: opts, now: opts.Now, log: opts.Logger}
}

// Run carries out tasks until ctx is cancelled.
func (r *Runner) Run(ctx context.Context) {
	go r.watchEvents(ctx)

	ticker := time.NewTicker(r.opts.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.Tick(ctx)
		}
	}
}

/*
Tick checks every clock-driven task once.

Exported so a test can step the schedule deterministically rather than waiting
on a real timer.
*/
func (r *Runner) Tick(ctx context.Context) {
	tasks, err := r.opts.Store.Tasks(ctx, r.opts.Server)
	if err != nil {
		r.log.Warn("could not read the task list", "server", r.opts.Server, "error", err)
		return
	}
	snap := r.opts.Poller.Snapshot()
	now := r.now()

	for _, task := range tasks {
		if !task.Enabled || task.Trigger.IsEvent() {
			continue
		}
		what := due(task, snap, now)

		if what.Remember {
			// The world changed in a way this task cares about, but not into a
			// state worth acting on. Recording it is what makes the next change
			// a change.
			if err := r.opts.Store.SetTaskKey(ctx, r.opts.Server, task.Name, task, what.Key); err != nil {
				r.log.Warn("could not update a task's bookkeeping",
					"server", r.opts.Server, "task", task.Name, "error", err)
			}
			continue
		}
		if !what.Fire && !what.StartClock {
			continue
		}

		/*
			The claim decides. Two ticks overlapping, or two panels on one
			database, must not both run the same restart. It is also what stamps
			the starting point of an interval that has never run, which is why a
			StartClock decision goes through it and then stops.
		*/
		won, err := r.opts.Store.ClaimTask(ctx, r.opts.Server, task.Name, task, what.Key, now)
		if err != nil {
			r.log.Warn("could not claim a task",
				"server", r.opts.Server, "task", task.Name, "error", err)
			continue
		}
		if !won || what.StartClock {
			continue
		}
		r.fire(ctx, task, nil)
	}
}

// watchEvents carries out the triggers that are about something happening
// rather than some time arriving.
func (r *Runner) watchEvents(ctx context.Context) {
	for ctx.Err() == nil {
		_, ch, cancel := r.opts.Feed.Subscribe(0)
		r.listen(ctx, ch)
		cancel()
	}
}

func (r *Runner) listen(ctx context.Context, ch <-chan events.Event) {
	for {
		select {
		case <-ctx.Done():
			return
		case e, ok := <-ch:
			if !ok {
				r.log.Warn("automation fell behind the event feed; resubscribing")
				return
			}
			switch e.Kind {
			case events.KindJoin:
				r.onEvent(ctx, store.TriggerJoin, e)
			case events.KindLeave:
				r.onEvent(ctx, store.TriggerLeave, e)
			case events.KindDeath:
				r.onEvent(ctx, store.TriggerDeath, e)
			}
		}
	}
}

// onEvent runs whichever tasks are waiting on something happening.
func (r *Runner) onEvent(ctx context.Context, kind store.TriggerKind, e events.Event) {
	tasks, err := r.opts.Store.Tasks(ctx, r.opts.Server)
	if err != nil {
		r.log.Warn("could not read the task list", "server", r.opts.Server, "error", err)
		return
	}
	for _, task := range tasks {
		if task.Enabled && task.Trigger == kind {
			r.fire(ctx, task, &e)
		}
	}
}

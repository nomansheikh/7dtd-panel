package automation

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/nomansheikh/7dtd-panel/internal/console"
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
		if !task.Enabled || task.Trigger == store.TriggerJoin {
			continue
		}
		ready, key := due(task, snap, now)
		if !ready {
			continue
		}
		// The claim decides. Two ticks overlapping, or two panels on one
		// database, must not both run the same restart.
		won, err := r.opts.Store.ClaimTask(ctx, r.opts.Server, task.Name, task, key, now)
		if err != nil {
			r.log.Warn("could not claim a task",
				"server", r.opts.Server, "task", task.Name, "error", err)
			continue
		}
		if !won {
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
			if e.Kind == events.KindJoin {
				r.onJoin(ctx, e)
			}
		}
	}
}

func (r *Runner) onJoin(ctx context.Context, e events.Event) {
	tasks, err := r.opts.Store.Tasks(ctx, r.opts.Server)
	if err != nil {
		r.log.Warn("could not read the task list", "server", r.opts.Server, "error", err)
		return
	}
	for _, task := range tasks {
		if task.Enabled && task.Trigger == store.TriggerJoin {
			r.fire(ctx, task, &e)
		}
	}
}

// runTimeout bounds one task end to end. A restart sequence with warnings is
// several commands, but none of them should take long.
const runTimeout = 60 * time.Second

/*
fire carries out a task's commands.

Every line is expanded and checked before any of them is sent, so a task whose
third line is malformed does none of the first two. They then run in order,
stopping at the first refusal: a restart that could not warn anybody should not
go on to shut the server down.
*/
func (r *Runner) fire(parent context.Context, task store.Task, e *events.Event) {
	ctx, cancel := context.WithTimeout(parent, runTimeout)
	defer cancel()

	run := store.TaskRun{Name: task.Name, RanAt: r.now()}

	lines, err := r.prepare(task, e)
	if err == nil {
		for _, line := range lines {
			if _, execErr := r.opts.Client.Execute(ctx, line); execErr != nil {
				err = fmt.Errorf("%s: %w", console.FirstWord(line), execErr)
				break
			}
		}
	}

	if err != nil {
		run.Error = err.Error()
		r.log.Warn("a task failed", "server", r.opts.Server, "task", task.Name, "error", err)
	} else {
		r.log.Info("a task ran", "server", r.opts.Server, "task", task.Name, "lines", len(lines))
	}

	if e == nil {
		// Clock-driven tasks are announced in the panel's own feed, so an
		// operator reading back through the console can see what happened
		// overnight without going looking for it.
		r.announce(task, run.Error)
	}
	if err := r.opts.Store.AddTaskRun(ctx, r.opts.Server, run); err != nil {
		r.log.Warn("could not record a task run", "server", r.opts.Server, "error", err)
	}
}

func (r *Runner) announce(task store.Task, failure string) {
	if failure != "" {
		r.opts.Feed.PublishStatus("task " + task.Name + " failed: " + failure)
		return
	}
	r.opts.Feed.PublishStatus("task " + task.Name + " ran")
}

// prepare expands and checks every line before one of them is sent.
func (r *Runner) prepare(task store.Task, e *events.Event) ([]string, error) {
	out := make([]string, 0, len(task.Commands))
	for _, line := range task.Commands {
		line = strings.TrimSpace(expand(line, e))
		if line == "" {
			continue
		}
		if err := console.Validate(line); err != nil {
			return nil, err
		}
		if !console.IsDestructiveAllowed(line, r.opts.AllowDestructive) {
			return nil, fmt.Errorf("%s is switched off in this panel", console.FirstWord(line))
		}
		out = append(out, line)
	}
	if len(out) == 0 {
		return nil, errors.New("nothing to run")
	}
	return out, nil
}

/*
expand fills in who, for the triggers that have a who.

Only the join trigger carries a player, and the values come from the log line
rather than from anything a player typed, so there is no free text here. A name
is still put through the same check the rest of the panel uses: a player called
`bob"; shutdown` is not something to paste into a command.
*/
func expand(line string, e *events.Event) string {
	if e == nil {
		return line
	}
	entity := ""
	if e.EntityID != nil {
		entity = strconv.Itoa(*e.EntityID)
	}
	return strings.NewReplacer(
		"{player}", safeWord(e.Player),
		"{entityid}", entity,
		"{platformid}", safeWord(e.PlatformID),
	).Replace(line)
}

// safeWord drops anything that could end an argument early or begin a second
// command. Names are chosen by players; nothing else here is.
func safeWord(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		case r == '_', r == '-', r == '.':
			return r
		}
		return -1
	}, s)
}

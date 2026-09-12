package automation

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/nomansheikh/7dtd-panel/internal/console"
	"github.com/nomansheikh/7dtd-panel/internal/events"
	"github.com/nomansheikh/7dtd-panel/internal/store"
)

// Carrying a task out, once something has decided it should run. The deciding
// is in due.go and runner.go; from here on the only question is what to send
// and what to say about it afterwards.

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

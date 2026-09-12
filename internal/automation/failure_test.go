package automation

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/nomansheikh/7dtd-panel/internal/store"
)

// What a task does when it cannot do what it was asked: a malformed line, a
// server that will not answer, and a command the operator switched off.

/* --------------------------------------------------------------- failure -- */

// A task whose third line is malformed does none of the first two.
func TestABadLineStopsTheWholeTask(t *testing.T) {
	r := newRig(t)
	r.save(t, store.Task{
		Name: "restart", Trigger: store.TriggerDaily, At: "05:00",
		Commands: []string{"saveworld", "say hello\nshutdown"},
	})

	r.now = time.Date(2026, 9, 12, 5, 0, 0, 0, time.Local)
	r.runner.Tick(t.Context())

	if got := r.client.commands(); len(got) != 0 {
		t.Errorf("ran %v; want nothing", got)
	}
	runs := r.runs(t)
	if len(runs) != 1 || runs[0].Error == "" {
		t.Errorf("runs = %+v; want one recorded failure", runs)
	}
}

// Stops at the first refusal: a restart that could not warn anybody should not
// go on to shut the server down.
func TestASequenceStopsAtTheFirstFailure(t *testing.T) {
	r := newRig(t)
	r.client.err = errors.New("connection refused")
	r.save(t, store.Task{
		Name: "restart", Trigger: store.TriggerDaily, At: "05:00",
		Commands: []string{`say "restarting"`, "shutdown"},
	})

	r.now = time.Date(2026, 9, 12, 5, 0, 0, 0, time.Local)
	r.runner.Tick(t.Context())

	got := r.client.commands()
	if len(got) != 1 {
		t.Fatalf("sent %v; want to stop after the first", got)
	}
	if strings.Contains(got[0], "shutdown") {
		t.Error("shut the server down after failing to warn anybody")
	}
}

/*
PANEL_ALLOW_DESTRUCTIVE is the operator's switch, and a schedule is not a way
around it.

With it on, a scheduled shutdown runs — that is the whole point of a nightly
restart. With it off, nothing destructive runs anywhere in the panel, and this
is nowhere special.
*/
func TestDestructiveTasksFollowTheOperatorsSwitch(t *testing.T) {
	r := newRig(t)
	r.runner.opts.AllowDestructive = false
	r.save(t, store.Task{
		Name: "restart", Trigger: store.TriggerDaily, At: "05:00",
		Commands: []string{"shutdown"},
	})

	r.now = time.Date(2026, 9, 12, 5, 0, 0, 0, time.Local)
	r.runner.Tick(t.Context())

	for _, c := range r.client.commands() {
		if c == "shutdown" {
			t.Error("shutdown ran with the switch off")
		}
	}
	if runs := r.runs(t); len(runs) != 1 || !strings.Contains(runs[0].Error, "switched off") {
		t.Errorf("runs = %+v; want the reason recorded", runs)
	}
}

// What the panel did overnight is worth being able to read back.
func TestRunsAreRecorded(t *testing.T) {
	r := newRig(t)
	r.save(t, store.Task{
		Name: "save", Trigger: store.TriggerDaily, At: "05:00",
		Commands: []string{"saveworld"},
	})

	r.now = time.Date(2026, 9, 12, 5, 0, 0, 0, time.Local)
	r.runner.Tick(t.Context())

	runs := r.runs(t)
	if len(runs) != 1 || runs[0].Name != "save" || runs[0].Error != "" {
		t.Fatalf("runs = %+v", runs)
	}
	if runs[0].RanAt.IsZero() {
		t.Error("the run has no time on it")
	}
}

// And in the panel's own feed, so it shows up in the console an operator is
// already reading.
func TestAClockTaskIsAnnouncedInTheFeed(t *testing.T) {
	r := newRig(t)
	r.save(t, store.Task{
		Name: "save", Trigger: store.TriggerDaily, At: "05:00",
		Commands: []string{"saveworld"},
	})

	r.now = time.Date(2026, 9, 12, 5, 0, 0, 0, time.Local)
	r.runner.Tick(t.Context())

	history, _, cancel := r.hub.Subscribe(10)
	defer cancel()
	var announced bool
	for _, e := range history {
		if strings.Contains(e.Message, "task save ran") {
			announced = true
		}
	}
	if !announced {
		t.Error("nothing in the feed said the task ran")
	}
}

package automation

import (
	"testing"
	"time"

	"github.com/nomansheikh/7dtd-panel/internal/store"
)

// A repeating task counts from the last time it ran, not from a wall clock.

func TestAnIntervalTaskRepeats(t *testing.T) {
	r := newRig(t)
	r.save(t, store.Task{
		Name: "save", Trigger: store.TriggerEvery, Minutes: 30,
		Commands: []string{"saveworld"},
	})

	// Never run, so the first tick only starts its clock.
	r.runner.Tick(t.Context())
	if n := len(r.client.commands()); n != 0 {
		t.Fatalf("ran %d times on the first tick, want 0", n)
	}

	// Claim the starting point the way the engine would, then step forward.
	task, err := r.db.Task(t.Context(), "test", "save")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.db.ClaimTask(t.Context(), "test", "save", task, "", r.now); err != nil {
		t.Fatal(err)
	}

	r.now = r.now.Add(31 * time.Minute)
	r.runner.Tick(t.Context())
	r.now = r.now.Add(31 * time.Minute)
	r.runner.Tick(t.Context())

	var saves int
	for _, c := range r.client.commands() {
		if c == "saveworld" {
			saves++
		}
	}
	if saves != 2 {
		t.Errorf("ran %d times over an hour of 30-minute intervals, want 2", saves)
	}
}

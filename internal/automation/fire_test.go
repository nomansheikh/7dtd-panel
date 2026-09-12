package automation

import (
	"testing"

	"github.com/nomansheikh/7dtd-panel/internal/events"
	"github.com/nomansheikh/7dtd-panel/internal/store"
)

// What happens once a task runs: the events that set one off, and everything
// that can go wrong while it is carrying its commands out.

/*
Through the real store: the disarm write has to stick, or a task that fires on
a change fires on every tick instead.
*/
func TestEmptyingTheServerSavesOnceAndRearms(t *testing.T) {
	r := newRig(t)
	r.save(t, store.Task{
		Name: "save-when-empty", Trigger: store.TriggerEmpty,
		Commands: []string{"saveworld"},
	})

	saves := func() int {
		n := 0
		for _, c := range r.client.commands() {
			if c == "saveworld" {
				n++
			}
		}
		return n
	}

	// Two players on, ticking away.
	r.runner.opts.Poller = fakePoller{snap: withPlayers(2)}
	r.runner.Tick(t.Context())
	if saves() != 0 {
		t.Fatal("saved while players were on")
	}

	// They leave. One save.
	r.runner.opts.Poller = fakePoller{snap: withPlayers(0)}
	for i := 0; i < 5; i++ {
		r.runner.Tick(t.Context())
	}
	if n := saves(); n != 1 {
		t.Fatalf("saved %d times across five empty ticks, want 1", n)
	}

	// Somebody joins, then leaves again: it fires a second time.
	r.runner.opts.Poller = fakePoller{snap: withPlayers(1)}
	r.runner.Tick(t.Context())
	r.runner.opts.Poller = fakePoller{snap: withPlayers(0)}
	r.runner.Tick(t.Context())
	if n := saves(); n != 2 {
		t.Errorf("saved %d times after a second emptying, want 2", n)
	}
}

// A death names whoever died, the same way a join names whoever joined.
func TestADeathTaskRunsForThePlayerWhoDied(t *testing.T) {
	r := newRig(t)
	r.save(t, store.Task{
		Name: "mock", Trigger: store.TriggerDeath,
		Commands: []string{`say "{player} died"`},
	})

	entity := 42
	r.runner.onEvent(t.Context(), store.TriggerDeath, events.Event{
		Kind: events.KindDeath, Player: "nullish", EntityID: &entity,
	})

	got := r.client.commands()
	if len(got) != 1 || got[0] != `say "nullish died"` {
		t.Errorf("commands = %v", got)
	}
}

// And a leave task does not run on a join.
func TestEventTasksOnlyRunForTheirOwnEvent(t *testing.T) {
	r := newRig(t)
	r.save(t, store.Task{
		Name: "farewell", Trigger: store.TriggerLeave,
		Commands: []string{`say "bye"`},
	})

	entity := 1
	r.runner.onEvent(t.Context(), store.TriggerJoin, events.Event{
		Kind: events.KindJoin, Player: "nullish", EntityID: &entity,
	})
	if got := r.client.commands(); len(got) != 0 {
		t.Errorf("a leave task ran on a join: %v", got)
	}

	r.runner.onEvent(t.Context(), store.TriggerLeave, events.Event{
		Kind: events.KindLeave, Player: "nullish", EntityID: &entity,
	})
	if got := r.client.commands(); len(got) != 1 {
		t.Errorf("commands = %v, want the farewell", got)
	}
}

package automation

import (
	"testing"
	"time"

	"github.com/nomansheikh/7dtd-panel/internal/sdtd"
	"github.com/nomansheikh/7dtd-panel/internal/state"
	"github.com/nomansheikh/7dtd-panel/internal/store"
)

// The game-clock trigger, and the guards that hold for every kind: nothing
// fires on a world the panel has not heard from, and nothing fires on the wrong
// sort of occasion.

func TestGametimeFiresOnceAnInGameDay(t *testing.T) {
	now := time.Now()
	at := time.Unix(1_700_000_000, 0).UTC()
	world := func(day, hour int) state.Snapshot {
		return state.Snapshot{
			Stats:   sdtd.ServerStats{GameTime: sdtd.GameTime{Days: day, Hours: hour}},
			StatsAt: at,
		}
	}
	// Every in-game evening at 21:00.
	task := store.Task{Trigger: store.TriggerGametime, At: "21:00"}

	if d := due(task, world(5, 20), now); d.Fire {
		t.Error("fired before the hour")
	}

	d := due(task, world(5, 21), now)
	if !d.Fire || d.Key != "d5" {
		t.Fatalf("at 21:00 on day 5 = %+v", d)
	}

	// The rest of that evening is the same evening.
	task.LastKey = d.Key
	for _, hour := range []int{21, 22, 23} {
		if again := due(task, world(5, hour), now); again.Fire {
			t.Errorf("fired twice on day 5, at %02d:00", hour)
		}
	}

	// The next day is a different one.
	if next := due(task, world(6, 21), now); !next.Fire || next.Key != "d6" {
		t.Errorf("day 6 = %+v, want a fresh fire", next)
	}
}

// Nothing fires on a world the panel has not heard from. A blank snapshot
// reports zero players, and zero players is not the same as "everybody left".
func TestNoChangeTriggerFiresWithoutAReading(t *testing.T) {
	for _, kind := range []store.TriggerKind{
		store.TriggerEmpty,
		store.TriggerUptime,
		store.TriggerBloodmoonOver,
		store.TriggerGametime,
	} {
		task := store.Task{Trigger: kind, Minutes: 360, At: "21:00"}
		if d := due(task, state.Snapshot{}, time.Now()); d.Fire {
			t.Errorf("%s fired on an empty snapshot", kind)
		}
	}
}

// The event triggers never come from the clock.
func TestEventTriggersAreNeverDueOnATick(t *testing.T) {
	for _, kind := range []store.TriggerKind{
		store.TriggerJoin, store.TriggerLeave, store.TriggerDeath,
	} {
		if !kind.IsEvent() {
			t.Errorf("%s is not marked as an event trigger", kind)
		}
		task := store.Task{Trigger: kind}
		if d := due(task, withPlayers(0), time.Now()); d.Fire {
			t.Errorf("%s fired on a clock tick", kind)
		}
	}
	for _, kind := range []store.TriggerKind{
		store.TriggerEvery, store.TriggerDaily, store.TriggerBloodmoon,
		store.TriggerGametime, store.TriggerUptime, store.TriggerEmpty,
		store.TriggerBloodmoonOver,
	} {
		if kind.IsEvent() {
			t.Errorf("%s is marked as an event trigger but runs on the clock", kind)
		}
	}
}

func TestValidateTriggerCoversTheNewOnes(t *testing.T) {
	cases := []struct {
		task store.Task
		ok   bool
	}{
		{store.Task{Trigger: store.TriggerGametime, At: "21:00"}, true},
		{store.Task{Trigger: store.TriggerGametime, At: "dusk"}, false},
		{store.Task{Trigger: store.TriggerUptime, Minutes: 360}, true},
		{store.Task{Trigger: store.TriggerUptime, Minutes: 0}, false},
		{store.Task{Trigger: store.TriggerEmpty}, true},
		{store.Task{Trigger: store.TriggerBloodmoonOver}, true},
		{store.Task{Trigger: store.TriggerLeave}, true},
		{store.Task{Trigger: store.TriggerDeath}, true},
	}
	for _, tc := range cases {
		t.Run(string(tc.task.Trigger), func(t *testing.T) {
			if err := ValidateTrigger(tc.task); (err == nil) != tc.ok {
				t.Errorf("ValidateTrigger = %v, want ok=%v", err, tc.ok)
			}
		})
	}
}

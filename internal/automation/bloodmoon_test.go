package automation

import (
	"testing"
	"time"

	"github.com/nomansheikh/7dtd-panel/internal/state"
	"github.com/nomansheikh/7dtd-panel/internal/store"
)

// The trigger no ordinary scheduler can express, and the arithmetic behind it.

/* ------------------------------------------------------------ blood moon -- */

/*
The one an ordinary scheduler cannot express.

Day 7 at 04:00, horde at dusk the same day — dawn plus eighteen hours of
daylight, so 18 game hours away. On a 60-minute day that is 45 real minutes;
on a 30-minute day it is 22.5. The same in-game moment, two different wall
clocks, which is exactly why this cannot be a crontab line.
*/
func TestBloodmoonIsMeasuredInRealTimeFromTheWorldsOwnDayLength(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()

	left, ok := untilBloodmoon(snap(7, 4, 7, 60), now)
	if !ok {
		t.Fatal("could not work out the time left")
	}
	if left < 44*time.Minute || left > 46*time.Minute {
		t.Errorf("on 60-minute days = %v, want about 45 minutes", left)
	}

	left, ok = untilBloodmoon(snap(7, 4, 7, 30), now)
	if !ok {
		t.Fatal("could not work out the time left")
	}
	if left < 22*time.Minute || left > 23*time.Minute {
		t.Errorf("on 30-minute days = %v, want about 22.5 minutes", left)
	}
}

func TestBloodmoonFiresInsideItsWindowAndOnlyOnce(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	task := store.Task{Trigger: store.TriggerBloodmoon, Minutes: 30}

	// Day 7 at 04:00 is 18 game hours out — 45 real minutes, too early for a
	// 30-minute warning.
	if due(task, snap(7, 4, 7, 60), now).Fire {
		t.Error("fired outside its window")
	}

	// Day 7 at 16:00 is 6 game hours out, which is 15 real minutes.
	what := due(task, snap(7, 16, 7, 60), now)
	if !what.Fire {
		t.Fatal("did not fire inside its window")
	}
	key := what.Key
	if key != "day7" {
		t.Errorf("key = %q, want the horde's day", key)
	}

	// It does not fire again for the same horde, however many polls follow.
	task.LastKey = key
	for _, hour := range []int{17, 18, 19, 20, 21} {
		if due(task, snap(7, hour, 7, 60), now).Fire {
			t.Errorf("fired twice for the same blood moon, at hour %d", hour)
		}
	}

	// The next one is a different horde.
	if next := due(task, snap(14, 16, 14, 60), now); !next.Fire || next.Key != "day14" {
		t.Errorf("next horde = %+v, want a fresh fire", next)
	}
}

// During the blood moon there is nothing left to warn about.
func TestBloodmoonDoesNotFireWhileItIsHappening(t *testing.T) {
	s := snap(7, 22, 7, 60)
	s.Bloodmoon.Active = true
	task := store.Task{Trigger: store.TriggerBloodmoon, Minutes: 30}
	if due(task, s, time.Now()).Fire {
		t.Error("warned about a blood moon that had already started")
	}
}

// Nothing fires on a world the panel has not heard from; a blank snapshot is
// not the same as "the horde is imminent".
func TestNothingFiresWithoutAReading(t *testing.T) {
	task := store.Task{Trigger: store.TriggerBloodmoon, Minutes: 30}
	if due(task, state.Snapshot{}, time.Now()).Fire {
		t.Error("fired on an empty snapshot")
	}
}

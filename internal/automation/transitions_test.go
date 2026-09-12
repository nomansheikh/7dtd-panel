package automation

import (
	"testing"
	"time"

	"github.com/nomansheikh/7dtd-panel/internal/sdtd"
	"github.com/nomansheikh/7dtd-panel/internal/state"
	"github.com/nomansheikh/7dtd-panel/internal/store"
)

/*
The triggers that fire on a change rather than at a time.

All of them share one hazard: the panel looks every thirty seconds, and the
world usually looks the same as it did. "The last player left" is worth acting
on once; "there is still nobody here" is the same fact arriving again, and a
save every thirty seconds until somebody joins would be the obvious way to get
this wrong. What stops it is the key the task records for the state it has
already seen.
*/

// withPlayers is a world with a player count and a reading behind it.
func withPlayers(n int) state.Snapshot {
	return state.Snapshot{
		Stats:   sdtd.ServerStats{Players: n},
		StatsAt: time.Unix(1_700_000_000, 0).UTC(),
	}
}

func TestEmptyFiresWhenTheLastPlayerLeavesAndNotAgain(t *testing.T) {
	now := time.Now()
	task := store.Task{Trigger: store.TriggerEmpty}

	// Somebody is on. Nothing to do, and nothing to remember either.
	if d := due(task, withPlayers(2), now); d.Fire || d.Remember {
		t.Errorf("with players on = %+v, want nothing", d)
	}

	// They leave.
	d := due(task, withPlayers(0), now)
	if !d.Fire || d.Key != "empty" {
		t.Fatalf("on emptying = %+v, want a fire", d)
	}

	// It stays empty. This is the one that would save every thirty seconds.
	task.LastKey = d.Key
	for i := 0; i < 5; i++ {
		if again := due(task, withPlayers(0), now); again.Fire {
			t.Fatal("fired again while the server was still empty")
		}
	}

	// Somebody joins: the task disarms, without running.
	rearm := due(task, withPlayers(1), now)
	if rearm.Fire {
		t.Error("fired when somebody joined")
	}
	if !rearm.Remember || rearm.Key != "" {
		t.Fatalf("on somebody joining = %+v, want it to disarm", rearm)
	}

	// And it can fire for the next emptying.
	task.LastKey = rearm.Key
	if next := due(task, withPlayers(0), now); !next.Fire {
		t.Error("did not fire the second time the server emptied")
	}
}

func TestUptimeFiresOnceTheServerHasBeenUpTooLong(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	up := func(d time.Duration) state.Snapshot {
		return state.Snapshot{
			Status:          state.StatusOnline,
			Uptime:          d,
			UptimeSampledAt: now,
			StatsAt:         now,
		}
	}
	// Six hours.
	task := store.Task{Trigger: store.TriggerUptime, Minutes: 360}

	if d := due(task, up(5*time.Hour), now); d.Fire {
		t.Error("fired below the line")
	}

	d := due(task, up(6*time.Hour), now)
	if !d.Fire || d.Key != "over" {
		t.Fatalf("at six hours = %+v, want a fire", d)
	}

	// Still up, still over: the restart it asked for may take a moment, and it
	// must not ask again every thirty seconds in the meantime.
	task.LastKey = d.Key
	if again := due(task, up(7*time.Hour), now); again.Fire {
		t.Error("asked twice while still over the line")
	}

	// The restart lands, uptime resets, and the task re-arms for next time.
	rearm := due(task, up(time.Minute), now)
	if rearm.Fire || !rearm.Remember || rearm.Key != "" {
		t.Fatalf("after a restart = %+v, want it to disarm", rearm)
	}
	task.LastKey = rearm.Key
	if next := due(task, up(6*time.Hour), now); !next.Fire {
		t.Error("did not fire after the next six hours")
	}
}

func TestBloodmoonOverFiresOnlyForAHordeItSawBegin(t *testing.T) {
	now := time.Now()
	at := time.Unix(1_700_000_000, 0).UTC()
	world := func(active bool) state.Snapshot {
		return state.Snapshot{
			Bloodmoon:   sdtd.Bloodmoon{Active: active},
			BloodmoonAt: at,
			StatsAt:     at,
		}
	}
	task := store.Task{Trigger: store.TriggerBloodmoonOver}

	// A panel started on a quiet afternoon has not just survived anything.
	if d := due(task, world(false), now); d.Fire {
		t.Error("congratulated everybody on surviving nothing")
	}

	// The horde arrives. Remembered, not acted on.
	d := due(task, world(true), now)
	if d.Fire {
		t.Error("fired when the horde arrived")
	}
	if !d.Remember || d.Key != "during" {
		t.Fatalf("during a horde = %+v, want it remembered", d)
	}

	// Still going: nothing new.
	task.LastKey = d.Key
	if again := due(task, world(true), now); again.Fire || again.Remember {
		t.Errorf("mid-horde = %+v, want nothing", again)
	}

	// It ends.
	over := due(task, world(false), now)
	if !over.Fire || over.Key != "" {
		t.Fatalf("when it ended = %+v, want a fire", over)
	}

	// And not again for the quiet week that follows.
	task.LastKey = over.Key
	for i := 0; i < 5; i++ {
		if quiet := due(task, world(false), now); quiet.Fire {
			t.Fatal("fired again during the quiet days after")
		}
	}
}

package automation

import (
	"testing"
	"time"

	"github.com/nomansheikh/7dtd-panel/internal/sdtd"
	"github.com/nomansheikh/7dtd-panel/internal/state"
	"github.com/nomansheikh/7dtd-panel/internal/store"
)

// snap builds a world: day/hour now, blood moon on nextDay, dayMinutes real
// minutes to a whole game day.
func snap(day, hour, nextDay, dayMinutes int) state.Snapshot {
	now := time.Unix(1_700_000_000, 0).UTC()
	return state.Snapshot{
		Stats:         sdtd.ServerStats{GameTime: sdtd.GameTime{Days: day, Hours: hour}},
		StatsAt:       now,
		DaylightHours: 18,
		DayMinutes:    dayMinutes,
		Bloodmoon: sdtd.Bloodmoon{
			GameTime: sdtd.GameTime{Days: day},
			Next:     sdtd.GameTime{Days: nextDay},
		},
		BloodmoonAt: now,
	}
}

/* ------------------------------------------------------------- intervals -- */

func TestEveryWaitsItsInterval(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	task := store.Task{Trigger: store.TriggerEvery, Minutes: 30, LastRunAt: now.Add(-29 * time.Minute)}

	if due(task, snap(1, 12, 7, 60), now).Fire {
		t.Error("fired a minute early")
	}
	task.LastRunAt = now.Add(-30 * time.Minute)
	if !due(task, snap(1, 12, 7, 60), now).Fire {
		t.Error("did not fire on its interval")
	}
}

// Switching on "every six hours" should not also mean "and right now".
func TestEveryDoesNotFireTheMomentItIsSwitchedOn(t *testing.T) {
	now := time.Now()
	task := store.Task{Trigger: store.TriggerEvery, Minutes: 360}
	if due(task, snap(1, 12, 7, 60), now).Fire {
		t.Error("fired immediately on a task that has never run")
	}
}

/* ----------------------------------------------------------------- daily -- */

func TestDailyFiresOncePastItsTime(t *testing.T) {
	day := func(h, m int) time.Time {
		return time.Date(2026, 9, 12, h, m, 0, 0, time.Local)
	}
	task := store.Task{Trigger: store.TriggerDaily, At: "05:00"}

	if due(task, snap(1, 12, 7, 60), day(4, 59)).Fire {
		t.Error("fired before its time")
	}

	what := due(task, snap(1, 12, 7, 60), day(5, 0))
	if !what.Fire {
		t.Fatal("did not fire at its time")
	}
	key := what.Key
	if key != "2026-09-12" {
		t.Errorf("key = %q, want the date", key)
	}

	// Having run, it does not run again today however often it is asked.
	task.LastKey = key
	task.LastRunAt = day(5, 0)
	for _, at := range []time.Time{day(5, 1), day(12, 0), day(23, 59)} {
		if due(task, snap(1, 12, 7, 60), at).Fire {
			t.Errorf("fired twice in a day, at %s", at.Format("15:04"))
		}
	}

	// Tomorrow it does.
	if !due(task, snap(1, 12, 7, 60), day(5, 0).AddDate(0, 0, 1)).Fire {
		t.Error("did not fire the next day")
	}
}

// A panel that was off when a task was due runs it late rather than skipping a
// day, because a missed restart is still worth doing.
func TestDailyRunsLateRatherThanSkipping(t *testing.T) {
	task := store.Task{Trigger: store.TriggerDaily, At: "05:00"}
	late := time.Date(2026, 9, 12, 9, 30, 0, 0, time.Local)
	if !due(task, snap(1, 12, 7, 60), late).Fire {
		t.Error("skipped a task it was late for")
	}
}

func TestDailyIgnoresANonsenseTime(t *testing.T) {
	for _, at := range []string{"", "5", "25:00", "05:60", "morning"} {
		task := store.Task{Trigger: store.TriggerDaily, At: at}
		if due(task, snap(1, 12, 7, 60), time.Now()).Fire {
			t.Errorf("fired on a time of %q", at)
		}
	}
}

/* -------------------------------------------------------------- triggers -- */

func TestValidateTrigger(t *testing.T) {
	cases := []struct {
		name string
		task store.Task
		ok   bool
	}{
		{"an interval", store.Task{Trigger: store.TriggerEvery, Minutes: 30}, true},
		{"a zero interval", store.Task{Trigger: store.TriggerEvery}, false},
		{"an interval of a year", store.Task{Trigger: store.TriggerEvery, Minutes: 525600}, false},
		{"a daily time", store.Task{Trigger: store.TriggerDaily, At: "05:00"}, true},
		{"a bad daily time", store.Task{Trigger: store.TriggerDaily, At: "5pm"}, false},
		{"a blood moon warning", store.Task{Trigger: store.TriggerBloodmoon, Minutes: 30}, true},
		{"a warning two days out", store.Task{Trigger: store.TriggerBloodmoon, Minutes: 3000}, false},
		{"on join", store.Task{Trigger: store.TriggerJoin}, true},
		{"nonsense", store.Task{Trigger: "whenever"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateTrigger(tc.task)
			if (err == nil) != tc.ok {
				t.Errorf("ValidateTrigger = %v, want ok=%v", err, tc.ok)
			}
		})
	}
}

// join has no schedule of its own; it fires from the event stream, never here.
func TestJoinIsNeverDueOnAClockTick(t *testing.T) {
	task := store.Task{Trigger: store.TriggerJoin}
	if due(task, snap(1, 12, 7, 60), time.Now()).Fire {
		t.Error("a join task fired on a clock tick")
	}
}

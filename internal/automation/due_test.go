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

	if ok, _ := due(task, snap(1, 12, 7, 60), now); ok {
		t.Error("fired a minute early")
	}
	task.LastRunAt = now.Add(-30 * time.Minute)
	if ok, _ := due(task, snap(1, 12, 7, 60), now); !ok {
		t.Error("did not fire on its interval")
	}
}

// Switching on "every six hours" should not also mean "and right now".
func TestEveryDoesNotFireTheMomentItIsSwitchedOn(t *testing.T) {
	now := time.Now()
	task := store.Task{Trigger: store.TriggerEvery, Minutes: 360}
	if ok, _ := due(task, snap(1, 12, 7, 60), now); ok {
		t.Error("fired immediately on a task that has never run")
	}
}

/* ----------------------------------------------------------------- daily -- */

func TestDailyFiresOncePastItsTime(t *testing.T) {
	day := func(h, m int) time.Time {
		return time.Date(2026, 9, 12, h, m, 0, 0, time.Local)
	}
	task := store.Task{Trigger: store.TriggerDaily, At: "05:00"}

	if ok, _ := due(task, snap(1, 12, 7, 60), day(4, 59)); ok {
		t.Error("fired before its time")
	}

	ok, key := due(task, snap(1, 12, 7, 60), day(5, 0))
	if !ok {
		t.Fatal("did not fire at its time")
	}
	if key != "2026-09-12" {
		t.Errorf("key = %q, want the date", key)
	}

	// Having run, it does not run again today however often it is asked.
	task.LastKey = key
	task.LastRunAt = day(5, 0)
	for _, at := range []time.Time{day(5, 1), day(12, 0), day(23, 59)} {
		if ok, _ := due(task, snap(1, 12, 7, 60), at); ok {
			t.Errorf("fired twice in a day, at %s", at.Format("15:04"))
		}
	}

	// Tomorrow it does.
	if ok, _ := due(task, snap(1, 12, 7, 60), day(5, 0).AddDate(0, 0, 1)); !ok {
		t.Error("did not fire the next day")
	}
}

// A panel that was off when a task was due runs it late rather than skipping a
// day, because a missed restart is still worth doing.
func TestDailyRunsLateRatherThanSkipping(t *testing.T) {
	task := store.Task{Trigger: store.TriggerDaily, At: "05:00"}
	late := time.Date(2026, 9, 12, 9, 30, 0, 0, time.Local)
	if ok, _ := due(task, snap(1, 12, 7, 60), late); !ok {
		t.Error("skipped a task it was late for")
	}
}

func TestDailyIgnoresANonsenseTime(t *testing.T) {
	for _, at := range []string{"", "5", "25:00", "05:60", "morning"} {
		task := store.Task{Trigger: store.TriggerDaily, At: at}
		if ok, _ := due(task, snap(1, 12, 7, 60), time.Now()); ok {
			t.Errorf("fired on a time of %q", at)
		}
	}
}

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
	if ok, _ := due(task, snap(7, 4, 7, 60), now); ok {
		t.Error("fired outside its window")
	}

	// Day 7 at 16:00 is 6 game hours out, which is 15 real minutes.
	ok, key := due(task, snap(7, 16, 7, 60), now)
	if !ok {
		t.Fatal("did not fire inside its window")
	}
	if key != "day7" {
		t.Errorf("key = %q, want the horde's day", key)
	}

	// It does not fire again for the same horde, however many polls follow.
	task.LastKey = key
	for _, hour := range []int{17, 18, 19, 20, 21} {
		if ok, _ := due(task, snap(7, hour, 7, 60), now); ok {
			t.Errorf("fired twice for the same blood moon, at hour %d", hour)
		}
	}

	// The next one is a different horde.
	if ok, k := due(task, snap(14, 16, 14, 60), now); !ok || k != "day14" {
		t.Errorf("next horde = %v %q, want a fresh fire", ok, k)
	}
}

// During the blood moon there is nothing left to warn about.
func TestBloodmoonDoesNotFireWhileItIsHappening(t *testing.T) {
	s := snap(7, 22, 7, 60)
	s.Bloodmoon.Active = true
	task := store.Task{Trigger: store.TriggerBloodmoon, Minutes: 30}
	if ok, _ := due(task, s, time.Now()); ok {
		t.Error("warned about a blood moon that had already started")
	}
}

// Nothing fires on a world the panel has not heard from; a blank snapshot is
// not the same as "the horde is imminent".
func TestNothingFiresWithoutAReading(t *testing.T) {
	task := store.Task{Trigger: store.TriggerBloodmoon, Minutes: 30}
	if ok, _ := due(task, state.Snapshot{}, time.Now()); ok {
		t.Error("fired on an empty snapshot")
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
	if ok, _ := due(task, snap(1, 12, 7, 60), time.Now()); ok {
		t.Error("a join task fired on a clock tick")
	}
}

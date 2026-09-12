/*
Package automation runs commands without anybody at a keyboard.

The game server has no scheduler, and no way to say "before the blood moon".
It runs a world and answers questions about it; anything that has to happen at
a time, or because something happened, needs somebody watching. The panel is
already watching — it polls the clock and holds the event stream — so it can be
that somebody.

Two of the four triggers are ordinary and one is not. every and daily are what
cron does. join is what a log watcher does. bloodmoon is neither: the horde
arrives on the game's clock, which runs at whatever rate this world is
configured for, so "half an hour before" is a moving wall-clock target that has
to be recomputed from the server's own day length every time it is asked. That
is the one an operator cannot rig up with crontab, and the reason this package
exists rather than a README section explaining how to write one.
*/
package automation

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/nomansheikh/7dtd-panel/internal/state"
	"github.com/nomansheikh/7dtd-panel/internal/store"
)

/*
due reports whether a task should run now, and the key to record if it does.

The key is what makes "again" mean something. For an interval it is empty and
the last-run time is enough. For a daily task it is the date, so a panel that
restarts at noon does not repeat the morning's task. For a blood moon it is the
day the horde lands on, so a task set half an hour before goes off once rather
than once per poll for thirty minutes.
*/
func due(task store.Task, snap state.Snapshot, now time.Time) (bool, string) {
	switch task.Trigger {
	case store.TriggerEvery:
		if task.Minutes <= 0 {
			return false, ""
		}
		if task.LastRunAt.IsZero() {
			// Never run. Start the clock now rather than firing immediately:
			// switching on "every six hours" should not mean "and also now".
			return false, ""
		}
		return now.Sub(task.LastRunAt) >= time.Duration(task.Minutes)*time.Minute, ""

	case store.TriggerDaily:
		at, ok := parseHHMM(task.At)
		if !ok {
			return false, ""
		}
		today := time.Date(now.Year(), now.Month(), now.Day(), at/60, at%60, 0, 0, now.Location())
		if now.Before(today) {
			return false, ""
		}
		key := today.Format("2006-01-02")
		if task.LastKey == key {
			return false, ""
		}
		// More than a day late means the panel was off when it was due. Run it
		// now rather than silently skipping to tomorrow: a missed restart is
		// worth doing late, and the alternative is a task that quietly never
		// happens on a panel that gets restarted a lot.
		return true, key

	case store.TriggerBloodmoon:
		left, ok := untilBloodmoon(snap, now)
		if !ok {
			return false, ""
		}
		if left > time.Duration(task.Minutes)*time.Minute {
			return false, ""
		}
		// Once per horde, named by the day it lands on.
		key := "day" + strconv.Itoa(snap.Bloodmoon.Next.Days)
		if task.LastKey == key {
			return false, ""
		}
		return true, key
	}
	return false, ""
}

/*
untilBloodmoon is how long, in real time, until the horde arrives.

The game reports the blood moon as a game day, and how long a game day takes in
real minutes. Neither alone answers the question an operator is asking, which is
how long they have. This is the arithmetic that turns one into the other, and
it is why a schedule written here beats one written in crontab: a server with
ninety-minute days and one with thirty-minute days need different wall-clock
warnings for the same in-game moment, and neither operator should have to work
that out.
*/
func untilBloodmoon(snap state.Snapshot, _ time.Time) (time.Duration, bool) {
	if snap.BloodmoonAt.IsZero() || snap.StatsAt.IsZero() {
		return 0, false
	}
	if snap.Bloodmoon.Active {
		return 0, false
	}
	dayMinutes := snap.DayMinutes
	if dayMinutes <= 0 {
		return 0, false
	}

	// The horde lands at dusk on its day, which is dawn plus the world's own
	// daylight length.
	daylight := snap.DaylightHours
	if daylight <= 0 || daylight >= 24 {
		daylight = 18
	}
	dusk := float64(dawnHour + daylight)

	nowGame := float64(snap.Stats.GameTime.Days)*24 +
		float64(snap.Stats.GameTime.Hours) +
		float64(snap.Stats.GameTime.Minutes)/60
	hordeGame := float64(snap.Bloodmoon.Next.Days)*24 + dusk

	gameHours := hordeGame - nowGame
	if gameHours <= 0 {
		return 0, false
	}
	// Game hours to real minutes, through the server's own day length.
	real := gameHours / 24 * float64(dayMinutes)
	return time.Duration(real * float64(time.Minute)), true
}

// dawnHour is when the game's day starts. Fixed in the game; the length of the
// day is not, and comes from the server's own setting.
const dawnHour = 4

// parseHHMM reads "HH:MM" into minutes past midnight.
func parseHHMM(s string) (int, bool) {
	h, m, found := strings.Cut(strings.TrimSpace(s), ":")
	if !found {
		return 0, false
	}
	hours, err := strconv.Atoi(h)
	if err != nil || hours < 0 || hours > 23 {
		return 0, false
	}
	mins, err := strconv.Atoi(m)
	if err != nil || mins < 0 || mins > 59 {
		return 0, false
	}
	return hours*60 + mins, true
}

// ValidateTrigger checks a trigger is one the engine can act on.
func ValidateTrigger(t store.Task) error {
	switch t.Trigger {
	case store.TriggerEvery:
		if t.Minutes < 1 || t.Minutes > 7*24*60 {
			return fmt.Errorf("an interval must be between a minute and a week")
		}
	case store.TriggerDaily:
		if _, ok := parseHHMM(t.At); !ok {
			return fmt.Errorf("a daily time must look like 05:00")
		}
	case store.TriggerBloodmoon:
		if t.Minutes < 0 || t.Minutes > 24*60 {
			return fmt.Errorf("a blood moon warning must be within a day of it")
		}
	case store.TriggerJoin:
	default:
		return fmt.Errorf("unknown trigger %q", t.Trigger)
	}
	return nil
}

package automation

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/nomansheikh/7dtd-panel/internal/state"
	"github.com/nomansheikh/7dtd-panel/internal/store"
)

// Reading the game's clock, and checking that a trigger is one the engine can
// act on. Kept apart from the decisions in due.go: one file works out what time
// it is in a world, the other works out what that means for a task.

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
	case store.TriggerGametime:
		if _, ok := parseHHMM(t.At); !ok {
			return fmt.Errorf("a game-time hour must look like 21:00")
		}
	case store.TriggerUptime:
		if t.Minutes < 1 || t.Minutes > 30*24*60 {
			return fmt.Errorf("an uptime must be between a minute and a month")
		}
	case store.TriggerEmpty, store.TriggerBloodmoonOver,
		store.TriggerJoin, store.TriggerLeave, store.TriggerDeath:
	default:
		return fmt.Errorf("unknown trigger %q", t.Trigger)
	}
	return nil
}

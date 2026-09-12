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
	"strconv"
	"time"

	"github.com/nomansheikh/7dtd-panel/internal/state"
	"github.com/nomansheikh/7dtd-panel/internal/store"
)

/*
decision is what a tick concludes about one task.

Key is what the world looks like to this task right now. Recording it even when
nothing runs is what lets a trigger fire on a change: the last player leaving is
interesting, and the next thirty seconds of nobody being there is not. A task
that has already recorded "empty" is one that has seen this, so it stays quiet
until somebody joins and the key clears.
*/
type decision struct {
	// Fire is whether the commands should run.
	Fire bool
	// Key is the bookkeeping to record. For an interval it is empty and the
	// last-run time carries the meaning; for everything else it names the
	// occasion, so that "again" means a different one.
	Key string
	// Remember records Key without running, which is how a change-driven task
	// disarms itself.
	Remember bool
}

/*
due reports what should happen to a task now.

Three shapes live here. An interval is answered by the clock alone. A daily or
game-time task names its occasion, so that being asked twice in the same one
changes nothing. And a change-driven task names the state of the world, firing
only on the way into a state worth acting on.
*/
func due(task store.Task, snap state.Snapshot, now time.Time) decision {
	switch task.Trigger {
	case store.TriggerEvery:
		if task.Minutes <= 0 || task.LastRunAt.IsZero() {
			// Never run. Start the clock now rather than firing immediately:
			// switching on "every six hours" should not mean "and also now".
			return decision{}
		}
		return decision{Fire: now.Sub(task.LastRunAt) >= time.Duration(task.Minutes)*time.Minute}

	case store.TriggerDaily:
		at, ok := parseHHMM(task.At)
		if !ok {
			return decision{}
		}
		today := time.Date(now.Year(), now.Month(), now.Day(), at/60, at%60, 0, 0, now.Location())
		if now.Before(today) {
			return decision{}
		}
		key := today.Format("2006-01-02")
		if task.LastKey == key {
			return decision{}
		}
		// More than a day late means the panel was off when it was due. Run it
		// now rather than silently skipping to tomorrow: a missed restart is
		// worth doing late, and the alternative is a task that quietly never
		// happens on a panel that gets restarted a lot.
		return decision{Fire: true, Key: key}

	case store.TriggerGametime:
		at, ok := parseHHMM(task.At)
		if !ok || snap.StatsAt.IsZero() {
			return decision{}
		}
		nowMins := snap.Stats.GameTime.Hours*60 + snap.Stats.GameTime.Minutes
		if nowMins < at {
			return decision{}
		}
		key := "d" + strconv.Itoa(snap.Stats.GameTime.Days)
		if task.LastKey == key {
			return decision{}
		}
		return decision{Fire: true, Key: key}

	case store.TriggerBloodmoon:
		left, ok := untilBloodmoon(snap, now)
		if !ok {
			return decision{}
		}
		if left > time.Duration(task.Minutes)*time.Minute {
			return decision{}
		}
		// Once per horde, named by the day it lands on.
		key := "day" + strconv.Itoa(snap.Bloodmoon.Next.Days)
		if task.LastKey == key {
			return decision{}
		}
		return decision{Fire: true, Key: key}

	case store.TriggerBloodmoonOver:
		if snap.BloodmoonAt.IsZero() {
			return decision{}
		}
		if snap.Bloodmoon.Active {
			// Remember that one is under way, so its ending is a change.
			return decision{Key: "during", Remember: task.LastKey != "during"}
		}
		if task.LastKey != "during" {
			// Not in one, and no horde was seen to end. Nothing to announce —
			// a panel started at noon has not just survived anything.
			return decision{}
		}
		return decision{Fire: true, Key: ""}

	case store.TriggerEmpty:
		if snap.StatsAt.IsZero() {
			return decision{}
		}
		if snap.Stats.Players > 0 {
			return decision{Key: "", Remember: task.LastKey != ""}
		}
		if task.LastKey == "empty" {
			return decision{}
		}
		return decision{Fire: true, Key: "empty"}

	case store.TriggerUptime:
		up, ok := snap.UptimeAt(now)
		if !ok || task.Minutes <= 0 {
			return decision{}
		}
		if up < time.Duration(task.Minutes)*time.Minute {
			// Below the line. A restart puts it back here, which re-arms the
			// task for the next time the server has been up too long.
			return decision{Key: "", Remember: task.LastKey != ""}
		}
		if task.LastKey == "over" {
			return decision{}
		}
		return decision{Fire: true, Key: "over"}
	}
	return decision{}
}

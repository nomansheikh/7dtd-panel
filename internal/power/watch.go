package power

import (
	"context"
	"time"

	"github.com/nomansheikh/7dtd-panel/internal/state"
)

/*
Watching to see whether the server comes back.

This is the part a person cannot do: after issuing a restart at five in the
morning nobody sits and refreshes a page for two minutes. The panel is already
polling, so it can answer the only question that matters — did it come back,
and how long was it gone — and say so plainly when it did not.

The answer is honest either way. A restart that never returns means nothing is
supervising the process, and the panel says exactly that rather than spinning.
*/

// howLongToWait is how long a server gets to come back before the panel calls
// it gone. Generous: a 7 Days to Die server loading a large world can take a
// couple of minutes before it answers again.
const howLongToWait = 5 * time.Minute

// checkEvery is how often the cached snapshot is read. Reading it is free — the
// poller is doing the work regardless.
const checkEvery = 2 * time.Second

/*
watch follows the server down and, with luck, back up.

It waits to see it actually go before it will believe it has returned. Without
that, a server that never went down at all — because the stop did not take —
would be reported as an instant, successful restart.
*/
func (c *Controller) watch(ctx context.Context, intent Intent) {
	c.set(func(s *Status) { s.Phase = PhaseWatching })

	deadline := c.opts.Now().Add(howLongToWait)
	sawItGo := false
	var wentAt time.Time

	for c.opts.Now().Before(deadline) {
		if err := c.opts.Sleep(ctx, checkEvery); err != nil {
			return
		}
		status := c.opts.Poller.Snapshot().Status

		if !sawItGo {
			if status != state.StatusOnline {
				sawItGo = true
				wentAt = c.opts.Now()
			}
			continue
		}
		if status == state.StatusOnline {
			down := int(c.opts.Now().Sub(wentAt).Seconds())
			c.set(func(s *Status) {
				s.Phase = PhaseBack
				s.DownSeconds = down
			})
			c.announce("power: the server is back after " + human(time.Duration(down)*time.Second))
			return
		}
	}

	// It did not come back inside the window. What that means depends entirely
	// on what was asked for.
	// Observed on a live server: a Docker restart policy is not always enough.
	// It fires when the container's main process exits, and a server run by a
	// wrapper script can stop the game while the wrapper keeps running — so the
	// container reads as healthy and nothing restarts. Naming that here saves
	// somebody the twenty minutes it took to work out the first time.
	note := "Nothing brought it back within five minutes. A restart needs something " +
		"else to start it again. If it runs under Docker, check the container is " +
		"actually restarting: a policy only fires when the container's main process " +
		"exits, and a wrapper script that keeps running will hide a stopped game."
	if intent == IntentStop {
		note = "The server is stopped."
	}
	if !sawItGo {
		note = "The panel never saw it go down. The stop may not have taken — " +
			"check the server itself."
	}
	c.set(func(s *Status) {
		s.Phase = PhaseGone
		s.Note = note
	})
	c.announce("power: " + note)
}

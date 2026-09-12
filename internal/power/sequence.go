package power

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/nomansheikh/7dtd-panel/internal/console"
)

/*
The sequence: warn, save, stop, then watch.

It runs on a context of its own rather than the request's, because the person
who starts a restart closes the tab and goes to bed. A fifteen-minute countdown
that died with the browser would warn everybody and then never shut anything
down, which is the worst of both.
*/

// warningsFor is when to tell people, given how long they have.
//
// Five points at most, thinning out as the time grows: somebody wants a warning
// at fifteen minutes and at one minute, and nothing in between is worth the
// interruption.
func warningsFor(total time.Duration) []time.Duration {
	candidates := []time.Duration{
		60 * time.Minute, 30 * time.Minute, 15 * time.Minute, 10 * time.Minute,
		5 * time.Minute, time.Minute,
	}
	var out []time.Duration
	for _, c := range candidates {
		if c <= total {
			out = append(out, c)
		}
	}
	return out
}

// ErrBusy is returned when a sequence is already running.
var ErrBusy = errors.New("a power sequence is already running")

/*
Start begins a countdown and returns immediately.

The returned error is only about starting: everything after it is reported
through Status, because by then nobody is waiting on an HTTP response.
*/
func (c *Controller) Start(intent Intent, countdown time.Duration, reason string) error {
	if !c.opts.AllowDestructive {
		return errors.New("shutdown is switched off in this panel (PANEL_ALLOW_DESTRUCTIVE)")
	}

	c.mu.Lock()
	if c.status.Phase != PhaseIdle && c.status.Phase != PhaseBack &&
		c.status.Phase != PhaseGone && c.status.Phase != PhaseFailed &&
		c.status.Phase != PhaseCancelled {
		c.mu.Unlock()
		return ErrBusy
	}
	now := c.opts.Now()
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	c.status = Status{
		Phase:     PhaseCountdown,
		Intent:    intent,
		StartedAt: now,
		StopAt:    now.Add(countdown),
	}
	c.mu.Unlock()

	go c.run(ctx, intent, countdown, reason)
	return nil
}

// Cancel calls off a sequence that has not stopped the server yet.
func (c *Controller) Cancel() bool {
	c.mu.Lock()
	phase, cancel := c.status.Phase, c.cancel
	c.mu.Unlock()

	// Past the point of stopping there is nothing to call off: the server is
	// already going down, and pretending otherwise would be a lie.
	if cancel == nil || (phase != PhaseCountdown && phase != PhaseSaving) {
		return false
	}
	cancel()
	return true
}

func (c *Controller) run(ctx context.Context, intent Intent, countdown time.Duration, reason string) {
	defer func() {
		c.mu.Lock()
		c.cancel = nil
		c.mu.Unlock()
	}()

	if err := c.countdown(ctx, intent, countdown, reason); err != nil {
		if errors.Is(err, context.Canceled) {
			c.set(func(s *Status) { *s = Status{Phase: PhaseCancelled, Intent: intent} })
			c.say("The " + string(intent) + " was called off.")
			c.announce("power: " + string(intent) + " cancelled")
			return
		}
		c.fail(err)
		return
	}

	// A save before the lights go out. Worth doing even though the server saves
	// on shutdown: if the stop wedges, this is the last known good world.
	c.set(func(s *Status) { s.Phase = PhaseSaving })
	if _, err := c.opts.Client.Execute(ctx, console.SaveWorld()); err != nil {
		c.fail(fmt.Errorf("saving the world: %w", err))
		return
	}

	c.set(func(s *Status) { s.Phase = PhaseStopping })
	if _, err := c.opts.Client.Execute(ctx, console.Shutdown()); err != nil {
		c.fail(fmt.Errorf("stopping the server: %w", err))
		return
	}
	c.announce("power: " + string(intent) + " — the server was told to stop")

	c.watch(context.Background(), intent)
}

// countdown warns at each point, then waits out whatever is left.
func (c *Controller) countdown(ctx context.Context, intent Intent, total time.Duration, reason string) error {
	verb := "restarting"
	if intent == IntentStop {
		verb = "shutting down"
	}

	elapsed := time.Duration(0)
	for _, at := range warningsFor(total) {
		wait := total - at - elapsed
		if err := c.opts.Sleep(ctx, wait); err != nil {
			return err
		}
		elapsed += wait

		message := fmt.Sprintf("Server %s in %s.", verb, human(at))
		if reason != "" {
			message += " " + reason
		}
		c.say(message)
	}
	return c.opts.Sleep(ctx, total-elapsed)
}

// say broadcasts to everybody on the server, and never fails the sequence: a
// restart that could not be announced should still happen.
func (c *Controller) say(message string) {
	command, err := console.Say(message)
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, _ = c.opts.Client.Execute(ctx, command)
}

func (c *Controller) announce(message string) {
	if c.opts.Announce != nil {
		c.opts.Announce.PublishStatus(message)
	}
}

func (c *Controller) fail(err error) {
	c.set(func(s *Status) {
		s.Phase = PhaseFailed
		s.Problem = err.Error()
	})
	c.announce("power: " + err.Error())
}

// human renders a stretch of time the way somebody would say it.
func human(d time.Duration) string {
	switch {
	case d < time.Minute:
		s := int(d / time.Second)
		if s == 1 {
			return "1 second"
		}
		return fmt.Sprintf("%d seconds", s)
	case d < time.Hour:
		m := int(d / time.Minute)
		if m == 1 {
			return "1 minute"
		}
		return fmt.Sprintf("%d minutes", m)
	}
	return "1 hour"
}

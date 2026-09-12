package power

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nomansheikh/7dtd-panel/internal/state"
)

/*
What the panel says afterwards, which is the only part anybody reads.

A restart at five in the morning is watched by nobody. The question it has to
answer unasked is "did it come back", and the answer has to be true in the case
where it did not.
*/

func TestItReportsHowLongTheServerWasGone(t *testing.T) {
	r := newRig(t)
	// Tick 1 is the watch loop's first look: the server has gone. By tick 4 it
	// is answering again.
	r.onTick = func(tick int) {
		switch tick {
		case 1:
			r.world.becomes(state.StatusOffline)
		case 4:
			r.world.becomes(state.StatusOnline)
		}
	}

	if err := r.c.Start(IntentRestart, 0, ""); err != nil {
		t.Fatal(err)
	}
	got := r.settle(t, PhaseBack, PhaseGone)

	if got.Phase != PhaseBack {
		t.Fatalf("phase = %q, want back. note: %s", got.Phase, got.Note)
	}
	// Three ticks of two seconds between going and returning.
	if got.DownSeconds != 6 {
		t.Errorf("downtime = %ds, want 6", got.DownSeconds)
	}
}

/*
A restart that never returns means nothing is supervising the process, and the
panel says that rather than spinning forever or quietly calling it a success.
*/
func TestARestartThatDoesNotComeBackSaysWhy(t *testing.T) {
	r := newRig(t)
	// It goes, and never returns.
	r.onTick = func(tick int) {
		if tick == 1 {
			r.world.becomes(state.StatusOffline)
		}
	}

	if err := r.c.Start(IntentRestart, 0, ""); err != nil {
		t.Fatal(err)
	}
	got := r.settle(t, PhaseGone, PhaseBack)

	if got.Phase != PhaseGone {
		t.Fatalf("phase = %q, want gone", got.Phase)
	}
	if !strings.Contains(got.Note, "needs something else to start it again") {
		t.Errorf("note = %q; it should say why it did not come back", got.Note)
	}
}

// The same ending is a success when a stop was what was asked for.
func TestAStopThatStaysDownIsNotAFailure(t *testing.T) {
	r := newRig(t)
	r.onTick = func(tick int) {
		if tick == 1 {
			r.world.becomes(state.StatusOffline)
		}
	}

	if err := r.c.Start(IntentStop, 0, ""); err != nil {
		t.Fatal(err)
	}
	got := r.settle(t, PhaseGone, PhaseBack)

	if got.Note != "The server is stopped." {
		t.Errorf("note = %q, want a plain statement that it stopped", got.Note)
	}
	if strings.Contains(got.Note, "supervis") {
		t.Error("a stop was explained as a failed restart")
	}
}

/*
A stop that did not take is not a restart that has not finished.

The panel will not claim a server came back unless it saw it leave, or a
shutdown that silently failed would read as an instant, flawless restart.
*/
func TestItWillNotClaimSuccessIfItNeverSawTheServerGo(t *testing.T) {
	r := newRig(t)
	// The world stays online throughout: the stop did nothing.
	if err := r.c.Start(IntentRestart, 0, ""); err != nil {
		t.Fatal(err)
	}
	got := r.settle(t, PhaseGone, PhaseBack)

	if got.Phase == PhaseBack {
		t.Fatal("reported a successful restart for a server that never went down")
	}
	if !strings.Contains(got.Note, "never saw it go down") {
		t.Errorf("note = %q; it should say the stop may not have taken", got.Note)
	}
}

/* ---------------------------------------------------------------- guards -- */

func TestItRefusesWhenShutdownIsSwitchedOff(t *testing.T) {
	r := newRig(t)
	r.c.opts.AllowDestructive = false

	err := r.c.Start(IntentRestart, 0, "")
	if err == nil {
		t.Fatal("started with PANEL_ALLOW_DESTRUCTIVE off")
	}
	if !strings.Contains(err.Error(), "PANEL_ALLOW_DESTRUCTIVE") {
		t.Errorf("error = %q; it should name the switch", err)
	}
	if len(r.client.commands()) != 0 {
		t.Errorf("sent %v", r.client.commands())
	}
}

func TestOnlyOneSequenceAtATime(t *testing.T) {
	r := newRig(t)
	// A sleep that blocks until released, so the first sequence stays running.
	release := make(chan struct{})
	var once sync.Once
	r.c.opts.Sleep = func(ctx context.Context, d time.Duration) error {
		once.Do(func() { <-release })
		return ctx.Err()
	}

	if err := r.c.Start(IntentRestart, time.Minute, ""); err != nil {
		t.Fatal(err)
	}
	if err := r.c.Start(IntentRestart, time.Minute, ""); err != ErrBusy {
		t.Errorf("second start = %v, want ErrBusy", err)
	}
	close(release)
}

// Calling it off during the countdown stops it and tells everybody.
func TestCancellingDuringTheCountdown(t *testing.T) {
	r := newRig(t)
	release := make(chan struct{})
	var once sync.Once
	r.c.opts.Sleep = func(ctx context.Context, d time.Duration) error {
		once.Do(func() { <-release })
		return ctx.Err()
	}

	if err := r.c.Start(IntentRestart, 15*time.Minute, ""); err != nil {
		t.Fatal(err)
	}
	if !r.c.Cancel() {
		t.Fatal("Cancel reported nothing to cancel")
	}
	close(release)

	got := r.settle(t, PhaseCancelled, PhaseGone, PhaseBack)
	if got.Phase != PhaseCancelled {
		t.Fatalf("phase = %q, want cancelled", got.Phase)
	}
	for _, c := range r.client.commands() {
		if c == "shutdown" {
			t.Error("shut the server down after being cancelled")
		}
	}
}

// Once the stop has gone in there is nothing to call off, and saying otherwise
// would be a lie.
func TestCancellingAfterTheStopDoesNothing(t *testing.T) {
	r := newRig(t)
	if err := r.c.Start(IntentRestart, 0, ""); err != nil {
		t.Fatal(err)
	}
	r.settle(t, PhaseGone, PhaseBack)

	if r.c.Cancel() {
		t.Error("claimed to cancel a server that had already been stopped")
	}
}

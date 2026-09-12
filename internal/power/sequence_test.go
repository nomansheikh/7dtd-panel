package power

import (
	"strings"
	"testing"
	"time"
)

// The countdown: what gets said, in what order, and what runs after it.

/* ------------------------------------------------------------ the warnings -- */

func TestACountdownWarnsThenSavesThenStops(t *testing.T) {
	r := newRig(t)
	// It never comes back, which is not what is under test here.
	if err := r.c.Start(IntentRestart, 15*time.Minute, "Nightly maintenance."); err != nil {
		t.Fatal(err)
	}
	r.settle(t, PhaseGone, PhaseBack)

	got := r.client.commands()
	var said []string
	for _, c := range got {
		if strings.HasPrefix(c, "say ") {
			said = append(said, c)
		}
	}
	// Fifteen minutes gets warnings at 15, 10, 5 and 1.
	if len(said) != 4 {
		t.Fatalf("warnings = %v, want four", said)
	}
	for i, want := range []string{"15 minutes", "10 minutes", "5 minutes", "1 minute"} {
		if !strings.Contains(said[i], want) {
			t.Errorf("warning %d = %q, want %q", i, said[i], want)
		}
	}
	if !strings.Contains(said[0], "Nightly maintenance.") {
		t.Errorf("the reason was not passed on: %q", said[0])
	}

	// Save before stop, and both after the warnings.
	saveAt, stopAt := indexOf(got, "saveworld"), indexOf(got, "shutdown")
	if saveAt < 0 || stopAt < 0 || saveAt > stopAt {
		t.Errorf("commands = %v, want a saveworld then a shutdown", got)
	}
}

// A stop says so. Telling people the server is restarting when it is not
// coming back is the one thing this must not do.
func TestAStopSaysStopping(t *testing.T) {
	r := newRig(t)
	if err := r.c.Start(IntentStop, 5*time.Minute, ""); err != nil {
		t.Fatal(err)
	}
	r.settle(t, PhaseGone, PhaseBack)

	for _, c := range r.client.commands() {
		if strings.HasPrefix(c, "say ") && strings.Contains(c, "restarting") {
			t.Errorf("a stop was announced as a restart: %q", c)
		}
	}
}

func TestNoCountdownStopsImmediately(t *testing.T) {
	r := newRig(t)
	if err := r.c.Start(IntentRestart, 0, ""); err != nil {
		t.Fatal(err)
	}
	r.settle(t, PhaseGone, PhaseBack)

	for _, c := range r.client.commands() {
		if strings.HasPrefix(c, "say ") {
			t.Errorf("warned during a countdown of zero: %q", c)
		}
	}
	if indexOf(r.client.commands(), "shutdown") < 0 {
		t.Error("never stopped the server")
	}
}

func indexOf(all []string, want string) int {
	for i, c := range all {
		if c == want {
			return i
		}
	}
	return -1
}

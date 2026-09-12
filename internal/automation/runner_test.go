package automation

import (
	"strings"
	"testing"
	"time"

	"github.com/nomansheikh/7dtd-panel/internal/events"
	"github.com/nomansheikh/7dtd-panel/internal/store"
)

/* ------------------------------------------------------------- the clock -- */

func TestADailyTaskRunsItsCommandsInOrder(t *testing.T) {
	r := newRig(t)
	r.save(t, store.Task{
		Name: "restart", Trigger: store.TriggerDaily, At: "05:00",
		Commands: []string{`say "restarting in 1 minute"`, "saveworld", "shutdown"},
	})

	r.now = time.Date(2026, 9, 12, 4, 59, 0, 0, time.Local)
	r.runner.Tick(t.Context())
	if got := r.client.commands(); len(got) != 0 {
		t.Fatalf("ran early: %v", got)
	}

	r.now = time.Date(2026, 9, 12, 5, 0, 0, 0, time.Local)
	r.runner.Tick(t.Context())

	want := []string{`say "restarting in 1 minute"`, "saveworld", "shutdown"}
	got := r.client.commands()
	for i, w := range want {
		if i >= len(got) || got[i] != w {
			t.Fatalf("commands = %v, want %v", got, want)
		}
	}
}

// The claim is what stops two ticks a second apart both running the restart.
func TestATaskRunsOnceHoweverOftenItIsTicked(t *testing.T) {
	r := newRig(t)
	r.save(t, store.Task{
		Name: "save", Trigger: store.TriggerDaily, At: "05:00",
		Commands: []string{"saveworld"},
	})

	r.now = time.Date(2026, 9, 12, 5, 0, 0, 0, time.Local)
	for i := 0; i < 5; i++ {
		r.runner.Tick(t.Context())
	}

	var saves int
	for _, c := range r.client.commands() {
		if c == "saveworld" {
			saves++
		}
	}
	if saves != 1 {
		t.Errorf("ran %d times across five ticks, want 1", saves)
	}
}

func TestADisabledTaskDoesNothing(t *testing.T) {
	r := newRig(t)
	if err := r.db.SaveTask(t.Context(), "test", store.Task{
		Name: "restart", Enabled: false, Trigger: store.TriggerDaily, At: "05:00",
		Commands: []string{"shutdown"},
	}, r.now); err != nil {
		t.Fatal(err)
	}

	r.now = time.Date(2026, 9, 12, 5, 0, 0, 0, time.Local)
	r.runner.Tick(t.Context())

	if got := r.client.commands(); len(got) != 0 {
		t.Errorf("a switched-off task ran %v", got)
	}
}

/* ------------------------------------------------------------ blood moon -- */

func TestABloodmoonWarningFiresOnTheWorldsClock(t *testing.T) {
	r := newRig(t)
	r.save(t, store.Task{
		Name: "horde-warning", Trigger: store.TriggerBloodmoon, Minutes: 30,
		Commands: []string{`say "the horde is coming"`},
	})

	// Day 7 at 04:00 is 45 real minutes out on a 60-minute day.
	r.runner.Tick(t.Context())
	if got := r.client.commands(); len(got) != 0 {
		t.Fatalf("warned too early: %v", got)
	}

	// Day 7 at 16:00 is 15 real minutes out.
	r.runner.opts.Poller = fakePoller{snap: snap(7, 16, 7, 60)}
	r.runner.Tick(t.Context())

	if got := r.client.commands(); len(got) != 1 {
		t.Fatalf("commands = %v, want one warning", got)
	}
}

/* ------------------------------------------------------------- on a join -- */

func TestAJoinTaskGreetsThePlayerWhoJoined(t *testing.T) {
	r := newRig(t)
	r.save(t, store.Task{
		Name: "welcome", Trigger: store.TriggerJoin,
		Commands: []string{`say "welcome {player}"`, "give {entityid} resourceWood 100"},
	})

	entity := 173
	r.runner.onEvent(t.Context(), store.TriggerJoin, events.Event{
		Kind: events.KindJoin, Player: "nullish", EntityID: &entity, PlatformID: "Steam_1",
	})

	got := r.client.commands()
	want := []string{`say "welcome nullish"`, "give 173 resourceWood 100"}
	for i, w := range want {
		if i >= len(got) || got[i] != w {
			t.Fatalf("commands = %v, want %v", got, want)
		}
	}
}

// Names are chosen by players. One with a quote in it must not reshape a line.
func TestAJoinTaskCannotBeReshapedByAName(t *testing.T) {
	r := newRig(t)
	r.save(t, store.Task{
		Name: "welcome", Trigger: store.TriggerJoin,
		Commands: []string{`say "welcome {player}"`},
	})

	entity := 1
	r.runner.onEvent(t.Context(), store.TriggerJoin, events.Event{
		Kind: events.KindJoin, Player: `bob"; shutdown; say "`, EntityID: &entity,
	})

	got := r.client.commands()
	if len(got) != 1 {
		t.Fatalf("commands = %v, want one", got)
	}
	// The word survives inside the quotes, which is fine — it is chat text, not
	// a command. What matters is that it is still one say with one quoted
	// argument, and that nothing escaped to start a second command.
	if !strings.HasPrefix(got[0], "say ") {
		t.Errorf("command is no longer a say: %q", got[0])
	}
	if strings.Count(got[0], `"`) != 2 {
		t.Errorf("command %q does not have exactly one quoted argument", got[0])
	}
	if strings.ContainsAny(got[0], ";\n") {
		t.Errorf("a separator got through: %q", got[0])
	}
}

// A clock task never fires from the feed, and a join task never from the clock.
func TestTriggersDoNotCrossOver(t *testing.T) {
	r := newRig(t)
	r.save(t, store.Task{
		Name: "welcome", Trigger: store.TriggerJoin, Commands: []string{"say hello"},
	})
	r.now = time.Date(2026, 9, 12, 5, 0, 0, 0, time.Local)
	r.runner.Tick(t.Context())
	if got := r.client.commands(); len(got) != 0 {
		t.Errorf("a join task fired on a tick: %v", got)
	}
}

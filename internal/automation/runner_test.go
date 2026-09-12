package automation

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nomansheikh/7dtd-panel/internal/events"
	"github.com/nomansheikh/7dtd-panel/internal/sdtd"
	"github.com/nomansheikh/7dtd-panel/internal/state"
	"github.com/nomansheikh/7dtd-panel/internal/store"
)

type fakeClient struct {
	mu   sync.Mutex
	sent []string
	err  error
}

func (c *fakeClient) Execute(_ context.Context, command string) (sdtd.CommandResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sent = append(c.sent, command)
	return sdtd.CommandResult{}, c.err
}

func (c *fakeClient) commands() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, len(c.sent))
	copy(out, c.sent)
	return out
}

type fakePoller struct{ snap state.Snapshot }

func (p fakePoller) Snapshot() state.Snapshot { return p.snap }

type rig struct {
	runner *Runner
	client *fakeClient
	db     *store.Store
	hub    *events.Hub
	now    time.Time
}

func newRig(t *testing.T) *rig {
	t.Helper()
	db, err := store.Open(t.Context(), t.TempDir()+"/panel.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	r := &rig{
		client: &fakeClient{},
		db:     db,
		hub:    events.NewHub(),
		now:    time.Date(2026, 9, 12, 4, 0, 0, 0, time.Local),
	}
	r.runner = New(Options{
		Server:           "test",
		Feed:             r.hub,
		Client:           r.client,
		Poller:           fakePoller{snap: snap(7, 4, 7, 60)},
		Store:            db,
		Now:              func() time.Time { return r.now },
		AllowDestructive: true,
	})
	return r
}

func (r *rig) save(t *testing.T, task store.Task) {
	t.Helper()
	task.Enabled = true
	if err := r.db.SaveTask(t.Context(), "test", task, r.now); err != nil {
		t.Fatal(err)
	}
}

func (r *rig) runs(t *testing.T) []store.TaskRun {
	t.Helper()
	got, err := r.db.RecentTaskRuns(t.Context(), "test", 50)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

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

func TestAnIntervalTaskRepeats(t *testing.T) {
	r := newRig(t)
	r.save(t, store.Task{
		Name: "save", Trigger: store.TriggerEvery, Minutes: 30,
		Commands: []string{"saveworld"},
	})

	// Never run, so the first tick only starts its clock.
	r.runner.Tick(t.Context())
	if n := len(r.client.commands()); n != 0 {
		t.Fatalf("ran %d times on the first tick, want 0", n)
	}

	// Claim the starting point the way the engine would, then step forward.
	task, err := r.db.Task(t.Context(), "test", "save")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.db.ClaimTask(t.Context(), "test", "save", task, "", r.now); err != nil {
		t.Fatal(err)
	}

	r.now = r.now.Add(31 * time.Minute)
	r.runner.Tick(t.Context())
	r.now = r.now.Add(31 * time.Minute)
	r.runner.Tick(t.Context())

	var saves int
	for _, c := range r.client.commands() {
		if c == "saveworld" {
			saves++
		}
	}
	if saves != 2 {
		t.Errorf("ran %d times over an hour of 30-minute intervals, want 2", saves)
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
	r.runner.onJoin(t.Context(), events.Event{
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
	r.runner.onJoin(t.Context(), events.Event{
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

/* --------------------------------------------------------------- failure -- */

// A task whose third line is malformed does none of the first two.
func TestABadLineStopsTheWholeTask(t *testing.T) {
	r := newRig(t)
	r.save(t, store.Task{
		Name: "restart", Trigger: store.TriggerDaily, At: "05:00",
		Commands: []string{"saveworld", "say hello\nshutdown"},
	})

	r.now = time.Date(2026, 9, 12, 5, 0, 0, 0, time.Local)
	r.runner.Tick(t.Context())

	if got := r.client.commands(); len(got) != 0 {
		t.Errorf("ran %v; want nothing", got)
	}
	runs := r.runs(t)
	if len(runs) != 1 || runs[0].Error == "" {
		t.Errorf("runs = %+v; want one recorded failure", runs)
	}
}

// Stops at the first refusal: a restart that could not warn anybody should not
// go on to shut the server down.
func TestASequenceStopsAtTheFirstFailure(t *testing.T) {
	r := newRig(t)
	r.client.err = errors.New("connection refused")
	r.save(t, store.Task{
		Name: "restart", Trigger: store.TriggerDaily, At: "05:00",
		Commands: []string{`say "restarting"`, "shutdown"},
	})

	r.now = time.Date(2026, 9, 12, 5, 0, 0, 0, time.Local)
	r.runner.Tick(t.Context())

	got := r.client.commands()
	if len(got) != 1 {
		t.Fatalf("sent %v; want to stop after the first", got)
	}
	if strings.Contains(got[0], "shutdown") {
		t.Error("shut the server down after failing to warn anybody")
	}
}

/*
PANEL_ALLOW_DESTRUCTIVE is the operator's switch, and a schedule is not a way
around it.

With it on, a scheduled shutdown runs — that is the whole point of a nightly
restart. With it off, nothing destructive runs anywhere in the panel, and this
is nowhere special.
*/
func TestDestructiveTasksFollowTheOperatorsSwitch(t *testing.T) {
	r := newRig(t)
	r.runner.opts.AllowDestructive = false
	r.save(t, store.Task{
		Name: "restart", Trigger: store.TriggerDaily, At: "05:00",
		Commands: []string{"shutdown"},
	})

	r.now = time.Date(2026, 9, 12, 5, 0, 0, 0, time.Local)
	r.runner.Tick(t.Context())

	for _, c := range r.client.commands() {
		if c == "shutdown" {
			t.Error("shutdown ran with the switch off")
		}
	}
	if runs := r.runs(t); len(runs) != 1 || !strings.Contains(runs[0].Error, "switched off") {
		t.Errorf("runs = %+v; want the reason recorded", runs)
	}
}

// What the panel did overnight is worth being able to read back.
func TestRunsAreRecorded(t *testing.T) {
	r := newRig(t)
	r.save(t, store.Task{
		Name: "save", Trigger: store.TriggerDaily, At: "05:00",
		Commands: []string{"saveworld"},
	})

	r.now = time.Date(2026, 9, 12, 5, 0, 0, 0, time.Local)
	r.runner.Tick(t.Context())

	runs := r.runs(t)
	if len(runs) != 1 || runs[0].Name != "save" || runs[0].Error != "" {
		t.Fatalf("runs = %+v", runs)
	}
	if runs[0].RanAt.IsZero() {
		t.Error("the run has no time on it")
	}
}

// And in the panel's own feed, so it shows up in the console an operator is
// already reading.
func TestAClockTaskIsAnnouncedInTheFeed(t *testing.T) {
	r := newRig(t)
	r.save(t, store.Task{
		Name: "save", Trigger: store.TriggerDaily, At: "05:00",
		Commands: []string{"saveworld"},
	})

	r.now = time.Date(2026, 9, 12, 5, 0, 0, 0, time.Local)
	r.runner.Tick(t.Context())

	history, _, cancel := r.hub.Subscribe(10)
	defer cancel()
	var announced bool
	for _, e := range history {
		if strings.Contains(e.Message, "task save ran") {
			announced = true
		}
	}
	if !announced {
		t.Error("nothing in the feed said the task ran")
	}
}

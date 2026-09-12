package chat

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

/* ------------------------------------------------------------- the fakes -- */

type fakeClient struct {
	mu   sync.Mutex
	sent []string
	// results are keyed by the first word of the command.
	results map[string]string
	err     error
	players []sdtd.Player
}

func (c *fakeClient) Execute(_ context.Context, command string) (sdtd.CommandResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sent = append(c.sent, command)
	if c.err != nil {
		return sdtd.CommandResult{}, c.err
	}
	name := command
	if i := strings.IndexByte(command, ' '); i > 0 {
		name = command[:i]
	}
	return sdtd.CommandResult{Result: c.results[name]}, nil
}

func (c *fakeClient) Players(context.Context) ([]sdtd.Player, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.players, c.err
}

// said returns every message the bot sent to a player, unwrapped.
func (c *fakeClient) said() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []string
	for _, command := range c.sent {
		if after, ok := strings.CutPrefix(command, "sayplayer "); ok {
			if i := strings.IndexByte(after, '"'); i >= 0 {
				out = append(out, strings.TrimSuffix(after[i+1:], `"`))
			}
		}
	}
	return out
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

/* ------------------------------------------------------------- the setup -- */

type harness struct {
	bot    *Bot
	client *fakeClient
	db     *store.Store
	hub    *events.Hub
	now    time.Time
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	db, err := store.Open(t.Context(), t.TempDir()+"/panel.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	h := &harness{
		client: &fakeClient{results: map[string]string{}},
		db:     db,
		hub:    events.NewHub(),
		now:    time.Unix(1_700_000_000, 0).UTC(),
	}
	h.bot = New(Options{
		Server: "test",
		Feed:   h.hub,
		Client: h.client,
		Poller: fakePoller{snap: state.Snapshot{
			Status:        state.StatusOnline,
			Stats:         sdtd.ServerStats{GameTime: sdtd.GameTime{Days: 12, Hours: 14, Minutes: 30}},
			StatsAt:       h.now,
			DaylightHours: 18,
			DayMinutes:    60,
			Bloodmoon: sdtd.Bloodmoon{
				GameTime: sdtd.GameTime{Days: 12},
				Next:     sdtd.GameTime{Days: 14},
			},
			BloodmoonAt: h.now,
		}},
		Store:    db,
		Announce: h.hub,
		Now:      func() time.Time { return h.now },
	})
	return h
}

// enable configures a command the way the panel's UI would.
func (h *harness) enable(t *testing.T, name string, audience store.Audience, cooldown int) {
	t.Helper()
	err := h.db.SaveChatCommand(t.Context(), "test", store.ChatCommand{
		Name: name, Enabled: true, Audience: audience, CooldownSeconds: cooldown,
	}, h.now)
	if err != nil {
		t.Fatal(err)
	}
}

// say drives one chat line through the bot, as the hub would.
func (h *harness) say(t *testing.T, text string) {
	t.Helper()
	req, ok := parse(chatEvent(173, "Steam_76561198803325430", "nullish", text))
	if !ok {
		t.Fatalf("%q was not recognised as a command", text)
	}
	h.bot.Handle(t.Context(), req)
}

func chatEvent(entityID int, platformID, player, text string) events.Event {
	return events.Event{
		Kind:       events.KindChat,
		Message:    text,
		Player:     player,
		EntityID:   &entityID,
		PlatformID: platformID,
	}
}

/* -------------------------------------------------------------- the loop -- */

// The one thing that must never happen: the panel answering its own broadcast.
func TestTheBotIgnoresItsOwnServersBroadcasts(t *testing.T) {
	// Verified live: say produces exactly this line back on the log.
	if _, ok := parse(chatEvent(-1, "-non-player-", "", "!day")); ok {
		t.Error("a server broadcast was taken as a player command")
	}
}

func TestParseRecognisesOnlyCommands(t *testing.T) {
	cases := []struct {
		name string
		in   events.Event
		want bool
	}{
		{"a command", chatEvent(1, "Steam_1", "a", "!day"), true},
		{"leading space", chatEvent(1, "Steam_1", "a", "  !day  "), true},
		{"arguments", chatEvent(1, "Steam_1", "a", "!kit starter"), true},
		{"ordinary chat", chatEvent(1, "Steam_1", "a", "day is long"), false},
		{"an exclamation", chatEvent(1, "Steam_1", "a", "!"), false},
		{"not chat", events.Event{Kind: events.KindLog, Message: "!day"}, false},
		{"no identity", events.Event{Kind: events.KindChat, Message: "!day"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, ok := parse(tc.in); ok != tc.want {
				t.Errorf("parse = %v, want %v", ok, tc.want)
			}
		})
	}
}

func TestParseLowercasesTheNameAndKeepsArguments(t *testing.T) {
	req, ok := parse(chatEvent(173, "Steam_1", "nullish", "!KIT  Starter extra"))
	if !ok {
		t.Fatal("not recognised")
	}
	if req.Name != "kit" {
		t.Errorf("name = %q, want kit", req.Name)
	}
	if len(req.Args) != 2 || req.Args[0] != "Starter" {
		t.Errorf("args = %v", req.Args)
	}
	if req.EntityID != 173 {
		t.Errorf("entity id = %d, want 173", req.EntityID)
	}
}

/* ------------------------------------------------------------- the gates -- */

// Nothing is on until an operator turns it on.
func TestADisabledCommandSaysNothingAtAll(t *testing.T) {
	h := newHarness(t)
	h.say(t, "!day")
	if got := h.client.commands(); len(got) != 0 {
		t.Errorf("sent %v; want silence", got)
	}
}

func TestAnUnknownCommandSaysNothing(t *testing.T) {
	h := newHarness(t)
	h.enable(t, "day", store.AudienceEveryone, 0)
	h.say(t, "!nuke")
	if got := h.client.commands(); len(got) != 0 {
		t.Errorf("sent %v; want silence", got)
	}
}

func TestAdminsOnlyRefusesEverybodyElse(t *testing.T) {
	h := newHarness(t)
	h.enable(t, "day", store.AudienceAdmins, 0)
	// A populated admin list, in the format a live server prints.
	h.client.results["admin"] = "Defined admins:\n      1: Steam_99999999999 (stored name: )\n"

	h.say(t, "!day")

	said := h.client.said()
	if len(said) != 1 || !strings.Contains(said[0], "Only admins") {
		t.Fatalf("said %v; want a refusal", said)
	}
}

func TestAdminsOnlyAllowsSomebodyOnTheList(t *testing.T) {
	h := newHarness(t)
	h.enable(t, "day", store.AudienceAdmins, 0)
	h.client.results["admin"] = "Defined admins:\n      1: Steam_76561198803325430 (stored name: )\n"

	h.say(t, "!day")

	said := h.client.said()
	if len(said) != 1 || !strings.HasPrefix(said[0], "Day 12") {
		t.Fatalf("said %v; want the day", said)
	}
}

// The admin list is a console command, so it must not be run per message.
func TestTheAdminListIsCached(t *testing.T) {
	h := newHarness(t)
	h.enable(t, "day", store.AudienceAdmins, 0)
	h.client.results["admin"] = "      1: Steam_76561198803325430 (stored name: )"

	h.say(t, "!day")
	h.say(t, "!day")

	var reads int
	for _, command := range h.client.commands() {
		if command == "admin list" {
			reads++
		}
	}
	if reads != 1 {
		t.Errorf("read the admin list %d times, want 1", reads)
	}

	// Once it is stale it is read again, so promoting somebody takes effect.
	h.now = h.now.Add(adminTTL + time.Second)
	h.say(t, "!day")
	reads = 0
	for _, command := range h.client.commands() {
		if command == "admin list" {
			reads++
		}
	}
	if reads != 2 {
		t.Errorf("read the admin list %d times after expiry, want 2", reads)
	}
}

/*
An empty admin list must not be mistaken for a populated one.

This is the exact output of a live server with nobody on the list, captured on
2026-09-12. It is a header of prose, and prose is where a loose pattern finds
identifiers that are not there — "SteamID" and "UserID" both appear in it.
*/
func TestAnEmptyAdminListLetsNobodyThrough(t *testing.T) {
	h := newHarness(t)
	h.enable(t, "day", store.AudienceAdmins, 0)
	h.client.results["admin"] = "Defined User Permissions:\n" +
		"  Level: UserID (Player name if online, stored name)\n" +
		"Defined Group Permissions:\n" +
		"  Normal,  Mods: SteamID (Stored name)\n"

	h.say(t, "!day")

	said := h.client.said()
	if len(said) != 1 || !strings.Contains(said[0], "Only admins") {
		t.Fatalf("said %v; want a refusal", said)
	}
}

func TestTheCooldownRefusesWithHowLongIsLeft(t *testing.T) {
	h := newHarness(t)
	h.enable(t, "day", store.AudienceEveryone, 600)

	h.say(t, "!day")
	h.now = h.now.Add(time.Minute)
	h.say(t, "!day")

	said := h.client.said()
	if len(said) != 2 {
		t.Fatalf("said %v", said)
	}
	if !strings.Contains(said[1], "9 minutes") {
		t.Errorf("second answer = %q; want the wait in minutes", said[1])
	}
}

// A command that failed must not cost the player their cooldown.
func TestAFailedCommandGivesTheCooldownBack(t *testing.T) {
	h := newHarness(t)
	h.enable(t, "players", store.AudienceEveryone, 600)

	allowed, _, err := h.db.TakeCooldown(t.Context(), "test", "Steam_76561198803325430", "players", 0, h.now)
	if err != nil || !allowed {
		t.Fatal(err)
	}

	h.client.err = errors.New("connection refused")
	h.say(t, "!players")
	h.client.err = nil

	allowed, left, err := h.db.TakeCooldown(t.Context(), "test", "Steam_76561198803325430", "players",
		10*time.Minute, h.now)
	if err != nil {
		t.Fatal(err)
	}
	if !allowed {
		t.Errorf("still on cooldown for %v after a failure", left)
	}
}

/* ----------------------------------------------------------- the answers -- */

func TestDay(t *testing.T) {
	h := newHarness(t)
	h.enable(t, "day", store.AudienceEveryone, 0)
	h.say(t, "!day")

	said := h.client.said()
	if len(said) != 1 {
		t.Fatalf("said %v", said)
	}
	// 14:30 on an 18-hour day: dusk is 22:00, so 7h30m of game time, which on
	// a 60-minute day is 18.75 real minutes, rounded up so nobody is told to
	// come back at a moment that has not arrived.
	for _, want := range []string{"Day 12", "14:30", "Dark in", "7h30m", "19 minutes"} {
		if !strings.Contains(said[0], want) {
			t.Errorf("answer %q is missing %q", said[0], want)
		}
	}
}

func TestBloodmoon(t *testing.T) {
	h := newHarness(t)
	h.enable(t, "bloodmoon", store.AudienceEveryone, 0)
	h.say(t, "!bloodmoon")

	said := h.client.said()
	if len(said) != 1 || !strings.Contains(said[0], "Day 14") || !strings.Contains(said[0], "2 days") {
		t.Fatalf("said %v; want day 14, two days away", said)
	}
}

func TestPlayers(t *testing.T) {
	h := newHarness(t)
	h.enable(t, "players", store.AudienceEveryone, 0)
	h.client.players = []sdtd.Player{
		{Name: "nullish", Online: true},
		{Name: "ghost", Online: false},
		{Name: "aria", Online: true},
	}
	h.say(t, "!players")

	said := h.client.said()
	if len(said) != 1 {
		t.Fatalf("said %v", said)
	}
	if !strings.Contains(said[0], "2 players online: aria, nullish") {
		t.Errorf("answer = %q", said[0])
	}
	if strings.Contains(said[0], "ghost") {
		t.Error("an offline player was listed")
	}
}

func TestHelpListsOnlyWhatThePlayerMayRun(t *testing.T) {
	h := newHarness(t)
	h.enable(t, "help", store.AudienceEveryone, 0)
	h.enable(t, "day", store.AudienceEveryone, 0)
	h.enable(t, "kit", store.AudienceAdmins, 0)
	h.client.results["admin"] = "Defined admins:\n"

	h.say(t, "!help")

	said := h.client.said()
	if len(said) != 1 {
		t.Fatalf("said %v", said)
	}
	if !strings.Contains(said[0], "!day") {
		t.Errorf("answer %q does not offer !day", said[0])
	}
	if strings.Contains(said[0], "!kit") {
		t.Errorf("answer %q offers a command this player would be refused", said[0])
	}
	if strings.Contains(said[0], "!bloodmoon") {
		t.Errorf("answer %q offers a command that is switched off", said[0])
	}
}

/* --------------------------------------------------------------- the kit -- */

func TestKitHandsOverEveryItem(t *testing.T) {
	h := newHarness(t)
	h.enable(t, "kit", store.AudienceEveryone, 3600)
	items := `[{"item":"resourceWood","count":500,"quality":0},
	           {"item":"gunHandgunT1Pistol","count":1,"quality":4}]`
	if err := h.db.SaveKit(t.Context(), "starter", items, h.now); err != nil {
		t.Fatal(err)
	}

	h.say(t, "!kit starter")

	want := []string{
		"give 173 resourceWood 500",
		"give 173 gunHandgunT1Pistol 1 4",
	}
	got := h.client.commands()
	for i, w := range want {
		if i >= len(got) || got[i] != w {
			t.Fatalf("commands = %v, want %v first", got, want)
		}
	}
	said := h.client.said()
	if len(said) != 1 || !strings.Contains(said[0], "501 items") {
		t.Errorf("said %v; want a count of what was given", said)
	}
}

// A kit with one bad name must hand over nothing, not half of itself.
func TestAKitWithAnUnsafeNameGivesNothing(t *testing.T) {
	h := newHarness(t)
	h.enable(t, "kit", store.AudienceEveryone, 0)
	items := `[{"item":"resourceWood","count":10},{"item":"wood; shutdown","count":1}]`
	if err := h.db.SaveKit(t.Context(), "bad", items, h.now); err != nil {
		t.Fatal(err)
	}

	h.say(t, "!kit bad")

	for _, command := range h.client.commands() {
		if strings.HasPrefix(command, "give") {
			t.Errorf("sent %q; want nothing given", command)
		}
	}
}

func TestAnUnknownKitNamesTheOnesThatExist(t *testing.T) {
	h := newHarness(t)
	h.enable(t, "kit", store.AudienceEveryone, 0)
	if err := h.db.SaveKit(t.Context(), "starter", `[]`, h.now); err != nil {
		t.Fatal(err)
	}

	h.say(t, "!kit nope")

	said := h.client.said()
	if len(said) != 1 || !strings.Contains(said[0], "starter") {
		t.Fatalf("said %v; want the kits that do exist", said)
	}
}

func TestKitWithoutANameExplainsItself(t *testing.T) {
	h := newHarness(t)
	h.enable(t, "kit", store.AudienceEveryone, 0)
	h.say(t, "!kit")

	said := h.client.said()
	if len(said) != 1 || !strings.Contains(said[0], "!kit starter") {
		t.Fatalf("said %v; want an example", said)
	}
}

/* ------------------------------------------------------------ the safety -- */

// The command set is fixed. Nothing an operator can configure adds to it, and
// nothing in it can end a session or discard world data.
func TestNoCommandIsDestructive(t *testing.T) {
	for _, spec := range Specs() {
		if spec.run == nil {
			t.Errorf("%s has no handler", spec.Name)
		}
		if !strings.HasPrefix(spec.Usage, Prefix) {
			t.Errorf("%s usage %q does not start with the prefix", spec.Name, spec.Usage)
		}
	}
	// Only one command may act at all.
	var acting []string
	for _, spec := range Specs() {
		if spec.Acts {
			acting = append(acting, spec.Name)
		}
	}
	if len(acting) != 1 || acting[0] != "kit" {
		t.Errorf("acting commands = %v, want [kit]", acting)
	}
}

// Names are attacker-chosen, so a reply carrying one must not be able to end
// the quoted argument early.
func TestRepliesCannotEscapeTheirQuotes(t *testing.T) {
	h := newHarness(t)
	h.enable(t, "players", store.AudienceEveryone, 0)
	h.client.players = []sdtd.Player{{Name: `a" shutdown "`, Online: true}}

	h.say(t, "!players")

	for _, command := range h.client.commands() {
		if strings.Count(command, `"`) != 2 {
			t.Errorf("command %q does not have exactly one quoted argument", command)
		}
	}
}

func TestSanitise(t *testing.T) {
	if got := sanitise(`say "hi"`); strings.ContainsAny(got, `"'`) {
		t.Errorf("sanitise kept a quote: %q", got)
	}
	if got := sanitise("a\nb"); strings.Contains(got, "\n") {
		t.Errorf("sanitise kept a newline: %q", got)
	}
	long := sanitise(strings.Repeat("x", 500))
	if len(long) > maxReply {
		t.Errorf("sanitise returned %d characters, want at most %d", len(long), maxReply)
	}
}

func TestHumanWait(t *testing.T) {
	cases := []struct {
		in   time.Duration
		want string
	}{
		{0, "1 second"},
		{1500 * time.Millisecond, "2 seconds"},
		{90 * time.Second, "2 minutes"},
		{time.Hour, "1 hour"},
		{time.Hour + 5*time.Minute, "1 hour 5 minutes"},
	}
	for _, tc := range cases {
		if got := humanWait(tc.in); got != tc.want {
			t.Errorf("humanWait(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

/* ---------------------------------------------------------------- the run -- */

// End to end through the real hub: a log line in, a reply out.
func TestRunAnswersAChatLineFromTheHub(t *testing.T) {
	h := newHarness(t)
	h.enable(t, "day", store.AudienceEveryone, 0)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		h.bot.Run(ctx)
	}()

	// Wait for the subscription before publishing, since the hub keeps no
	// backlog for the bot on purpose.
	waitFor(t, func() bool {
		subs, _, _ := h.hub.Stats()
		return subs > 0
	})

	h.hub.PublishLog(sdtd.LogEntry{
		ID:  1,
		Msg: "Chat (from 'Steam_76561198803325430', entity id '173', to 'Global'): 'nullish': !day",
	})

	waitFor(t, func() bool { return len(h.client.said()) > 0 })
	if said := h.client.said(); !strings.HasPrefix(said[0], "Day 12") {
		t.Errorf("said %q", said[0])
	}

	cancel()
	<-done
}

// And the same line from the server itself gets no answer.
func TestRunIgnoresTheServersOwnLine(t *testing.T) {
	h := newHarness(t)
	h.enable(t, "day", store.AudienceEveryone, 0)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go h.bot.Run(ctx)

	waitFor(t, func() bool {
		subs, _, _ := h.hub.Stats()
		return subs > 0
	})

	h.hub.PublishLog(sdtd.LogEntry{
		ID:  1,
		Msg: "Chat (from '-non-player-', entity id '-1', to 'Global'): !day",
	})
	// Then a real one, so the test waits on something that does arrive rather
	// than on a timeout that proves little.
	h.hub.PublishLog(sdtd.LogEntry{
		ID:  2,
		Msg: "Chat (from 'Steam_76561198803325430', entity id '173', to 'Global'): 'nullish': !day",
	})

	waitFor(t, func() bool { return len(h.client.said()) > 0 })
	time.Sleep(50 * time.Millisecond)
	if said := h.client.said(); len(said) != 1 {
		t.Errorf("said %v; want exactly one answer", said)
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timed out waiting")
}

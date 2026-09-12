package chat

import (
	"errors"
	"strings"
	"testing"

	"github.com/nomansheikh/7dtd-panel/internal/store"
)

// save writes a custom command the way the panel's UI would.
func (h *harness) custom(t *testing.T, c store.ChatCommand) {
	t.Helper()
	c.Kind = store.KindCustom
	c.Enabled = true
	if c.Audience == "" {
		c.Audience = store.AudienceEveryone
	}
	if err := h.db.SaveChatCommand(t.Context(), "test", c, h.now); err != nil {
		t.Fatal(err)
	}
}

/* ------------------------------------------------------- the plain cases -- */

// The commonest custom command of all: a canned answer, no game state touched.
func TestACustomCommandCanJustAnswer(t *testing.T) {
	h := newHarness(t)
	h.custom(t, store.ChatCommand{Name: "discord", Reply: "Join us: discord.gg/example"})

	h.say(t, "!discord")

	said := h.client.said()
	if len(said) != 1 || !strings.Contains(said[0], "discord.gg/example") {
		t.Fatalf("said %v", said)
	}
	// Nothing but the reply was sent.
	for _, command := range h.client.commands() {
		if !strings.HasPrefix(command, "sayplayer") {
			t.Errorf("a reply-only command ran %q", command)
		}
	}
}

func TestACustomCommandRunsItsLinesInOrder(t *testing.T) {
	h := newHarness(t)
	h.custom(t, store.ChatCommand{
		Name:     "starter",
		Reply:    "There you go, {player}.",
		Commands: []string{"give {entityid} resourceWood 500", "give {entityid} meleeToolAxeT1IronFireaxe 1 4"},
	})

	h.say(t, "!starter")

	got := h.client.commands()
	want := []string{
		"give 173 resourceWood 500",
		"give 173 meleeToolAxeT1IronFireaxe 1 4",
	}
	for i, w := range want {
		if i >= len(got) || got[i] != w {
			t.Fatalf("commands = %v, want %v first", got, want)
		}
	}
	if said := h.client.said(); len(said) != 1 || !strings.Contains(said[0], "nullish") {
		t.Errorf("said %v; want the player's name in the reply", said)
	}
}

// Acting with nothing written to say still says something, or the player is
// left wondering whether it worked.
func TestACustomCommandThatActsSilentlyStillConfirms(t *testing.T) {
	h := newHarness(t)
	h.custom(t, store.ChatCommand{Name: "heal", Commands: []string{"buff {entityid} buffMedicated"}})

	h.say(t, "!heal")

	if said := h.client.said(); len(said) != 1 || said[0] != "Done." {
		t.Errorf("said %v, want a confirmation", said)
	}
}

// A name an admin never wrote a row for is not a command, however it is typed.
func TestAnUnwrittenCustomNameIsSilent(t *testing.T) {
	h := newHarness(t)
	h.say(t, "!discord")
	if got := h.client.commands(); len(got) != 0 {
		t.Errorf("sent %v; want silence", got)
	}
}

/* ---------------------------------------------------------- substitution -- */

func TestExpandCommandFillsWhoAsked(t *testing.T) {
	req := Request{
		Player:     "nullish",
		EntityID:   173,
		PlatformID: "Steam_76561198803325430",
		Args:       []string{"resourceWood", "500"},
	}
	cases := []struct{ in, want string }{
		{"give {entityid} {arg1} {arg2}", "give 173 resourceWood 500"},
		{"ban add {platformid} 1 day", "ban add Steam_76561198803325430 1 day"},
		{"admin add {platformid} 2", "admin add Steam_76561198803325430 2"},
		{"spawnairdrop", "spawnairdrop"},
		// {args} is several words on purpose: more arguments to the same
		// command, which is what an admin means by writing it.
		{"give {entityid} {args}", "give 173 resourceWood 500"},
		// An argument that was not typed leaves a gap rather than a guess.
		{"give {entityid} {arg9}", "give 173 "},
	}
	for _, tc := range cases {
		got, err := expandCommand(tc.in, req)
		if err != nil {
			t.Errorf("expandCommand(%q) errored: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("expandCommand(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

/*
The line that runs has to be the line the admin wrote.

Not a judgement about what an operator may do on their own server — they can
already type any of this on the console page. It is that the author of a
command is the admin, and a player who can reshape the line has made themselves
the author.
*/
func TestPlayerTextCannotReshapeACommand(t *testing.T) {
	req := Request{Player: "nullish", EntityID: 173, PlatformID: "Steam_1"}
	for _, arg := range []string{
		`wood; shutdown`,
		`wood" "`,
		`wood shutdown`,
		"wood\nshutdown",
	} {
		req.Args = []string{arg}
		if _, err := expandCommand("give {entityid} {arg1} 1", req); err == nil {
			t.Errorf("argument %q was accepted into a command line", arg)
		}
	}
}

// A player's own name is not something they can be trusted to have chosen
// carefully, and it is not something the admin picked either.
func TestAPlayerNameCannotReshapeACommand(t *testing.T) {
	req := Request{Player: `bob"; shutdown`, EntityID: 173, PlatformID: "Steam_1"}
	if _, err := expandCommand("say {player} joined", req); err == nil {
		t.Error("a name with a quote in it was accepted into a command line")
	}
	// The same name in a reply is only a name; nothing there becomes a command.
	if got := expandReply("Welcome {player}", req); !strings.Contains(got, "bob") {
		t.Errorf("expandReply dropped the name: %q", got)
	}
}

// The refusal reaches the player, since it is the one failure they can fix.
func TestAnUnsafeArgumentTellsThePlayerWhy(t *testing.T) {
	h := newHarness(t)
	h.custom(t, store.ChatCommand{Name: "get", Commands: []string{"give {entityid} {arg1} 1"}})

	h.say(t, "!get wood;shutdown")

	said := h.client.said()
	if len(said) != 1 || !strings.Contains(said[0], "plain words") {
		t.Fatalf("said %v; want an explanation", said)
	}
	for _, command := range h.client.commands() {
		if strings.HasPrefix(command, "give") {
			t.Errorf("sent %q", command)
		}
	}
}

func TestErrUnsafeArgumentIsMatchable(t *testing.T) {
	_, err := expandCommand("give {entityid} {arg1}", Request{Args: []string{"a b"}})
	var unsafe *ErrUnsafeArgument
	if !errors.As(err, &unsafe) {
		t.Fatalf("error = %v, want an ErrUnsafeArgument", err)
	}
}

/* ------------------------------------------------------------- the gates -- */

// Nothing runs until one of the lines has been checked, so a command whose
// third line is bad does not do the first two.
func TestABadLineStopsTheWholeCommand(t *testing.T) {
	h := newHarness(t)
	h.custom(t, store.ChatCommand{
		Name: "half",
		Commands: []string{
			"give {entityid} resourceWood 10",
			"give {entityid} {arg1} 1",
		},
	})

	h.say(t, "!half wood;shutdown")

	for _, command := range h.client.commands() {
		if strings.HasPrefix(command, "give") {
			t.Errorf("sent %q; want nothing run", command)
		}
	}
}

/*
PANEL_ALLOW_DESTRUCTIVE is the operator's switch, and it means the same thing
here as it does on the console page.

Not the panel deciding what a server owner may do: with the switch on, this
runs. With it off, it does not run anywhere, and a chat trigger is not a way
around a setting they chose.
*/
func TestDestructiveLinesFollowTheOperatorsSwitch(t *testing.T) {
	t.Run("off", func(t *testing.T) {
		h := newHarness(t)
		h.bot.opts.AllowDestructive = false
		h.custom(t, store.ChatCommand{Name: "wipe", Commands: []string{"killall"}})

		h.say(t, "!wipe")

		for _, command := range h.client.commands() {
			if command == "killall" {
				t.Error("killall ran with the switch off")
			}
		}
	})

	t.Run("on", func(t *testing.T) {
		h := newHarness(t)
		h.bot.opts.AllowDestructive = true
		h.custom(t, store.ChatCommand{Name: "wipe", Commands: []string{"killall"}})

		h.say(t, "!wipe")

		var ran bool
		for _, command := range h.client.commands() {
			if command == "killall" {
				ran = true
			}
		}
		if !ran {
			t.Error("killall did not run with the switch on; it is the operator's call")
		}
	})
}

// The audience gate is the same one the built-ins use.
func TestACustomCommandCanBeHeldToAdmins(t *testing.T) {
	h := newHarness(t)
	h.custom(t, store.ChatCommand{
		Name:     "wipe",
		Audience: store.AudienceAdmins,
		Commands: []string{"say cleared"},
	})
	h.client.results["admin"] = "Defined admins:\n"

	h.say(t, "!wipe")

	said := h.client.said()
	if len(said) != 1 || !strings.Contains(said[0], "Only admins") {
		t.Fatalf("said %v; want a refusal", said)
	}
}

// A custom command that acts shows up in the panel's own feed, so an operator
// sees what the bot did on somebody else's say-so.
func TestActingCustomCommandsAreAnnounced(t *testing.T) {
	h := newHarness(t)
	h.custom(t, store.ChatCommand{Name: "heal", Commands: []string{"buff {entityid} buffMedicated"}})

	before, _, _ := h.hub.Stats()
	_ = before
	h.say(t, "!heal")

	history, _, cancel := h.hub.Subscribe(10)
	defer cancel()
	var announced bool
	for _, e := range history {
		if strings.Contains(e.Message, "ran !heal") {
			announced = true
		}
	}
	if !announced {
		t.Error("nothing in the feed said the command ran")
	}
}

func TestDeletingACustomCommandForgetsItsCooldowns(t *testing.T) {
	h := newHarness(t)
	h.custom(t, store.ChatCommand{Name: "daily", CooldownSeconds: 86400, Reply: "here"})

	h.say(t, "!daily")
	if said := h.client.said(); len(said) != 1 {
		t.Fatalf("said %v", said)
	}

	if err := h.db.DeleteChatCommand(t.Context(), "test", "daily"); err != nil {
		t.Fatal(err)
	}
	// Recreated under the same name, nobody is still waiting on the old one.
	h.custom(t, store.ChatCommand{Name: "daily", CooldownSeconds: 86400, Reply: "here"})

	h.say(t, "!daily")
	if said := h.client.said(); len(said) != 2 {
		t.Errorf("said %v; want the recreated command to answer", said)
	}
}

package api

import (
	"net/http"
	"testing"

	"github.com/nomansheikh/7dtd-panel/internal/sdtd"
	"github.com/nomansheikh/7dtd-panel/internal/servers"
	"github.com/nomansheikh/7dtd-panel/internal/state"
)

// commands reads the list back the way the UI does.
func (h *harness) commands(t *testing.T, cookie *http.Cookie) []chatCommandRow {
	t.Helper()
	rec := h.do(t, h.request(t, http.MethodGet, "/api/chat/commands", "", cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("listing returned %d: %s", rec.Code, rec.Body.String())
	}
	return decode[struct {
		Commands []chatCommandRow `json:"commands"`
	}](t, rec).Commands
}

func chatHarness(t *testing.T) (*harness, *http.Cookie) {
	t.Helper()
	h := newHarness(t, state.Snapshot{Status: state.StatusOnline})
	h.srv.Items = &fakeItems{items: []sdtd.Item{
		{Name: "resourceWood"},
		{Name: "gunHandgunT1Pistol"},
	}}
	return h, h.login(t, "admin", testPassword)
}

// The list is what the panel can answer, not what somebody has configured, so a
// fresh panel still shows every command with a way to turn it on.
func TestChatCommandsListsEverythingOffByDefault(t *testing.T) {
	h, cookie := chatHarness(t)

	rec := h.do(t, h.request(t, http.MethodGet, "/api/chat/commands", "", cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}

	body := decode[struct {
		Prefix   string           `json:"prefix"`
		Commands []chatCommandRow `json:"commands"`
	}](t, rec)

	if body.Prefix != "!" {
		t.Errorf("prefix = %q, want !", body.Prefix)
	}
	if len(body.Commands) == 0 {
		t.Fatal("no commands listed")
	}
	for _, c := range body.Commands {
		if c.Enabled {
			t.Errorf("%s is on before anybody turned it on", c.Name)
		}
		if c.Summary == "" || c.Usage == "" {
			t.Errorf("%s has nothing to show an operator: %+v", c.Name, c)
		}
	}
}

func TestChatCommandRoundTripsThroughTheAPI(t *testing.T) {
	h, cookie := chatHarness(t)

	rec := h.do(t, h.request(t, http.MethodPut, "/api/chat/commands/kit",
		`{"enabled":true,"audience":"admins","cooldownSeconds":900}`, cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}

	rec = h.do(t, h.request(t, http.MethodGet, "/api/chat/commands", "", cookie))
	for _, c := range decode[struct {
		Commands []chatCommandRow `json:"commands"`
	}](t, rec).Commands {
		if c.Name != "kit" {
			continue
		}
		if !c.Enabled || c.Audience != "admins" || c.CooldownSeconds != 900 {
			t.Errorf("kit came back as %+v", c)
		}
		if !c.Acts {
			t.Error("kit is not marked as acting; the UI would not warn about it")
		}
		return
	}
	t.Fatal("kit was not in the list")
}

/*
An admin can write their own, and the panel does not second-guess what it runs.

This is somebody's own server, and they already have a console page here that
runs anything they type. The earlier version of this test asserted the
opposite — that only the built-in five could exist — which was a rule that
applied nowhere else in the product.
*/
func TestAnAdminCanWriteTheirOwnCommand(t *testing.T) {
	h, cookie := chatHarness(t)

	rec := h.do(t, h.request(t, http.MethodPut, "/api/chat/commands/discord",
		`{"enabled":true,"audience":"everyone","cooldownSeconds":30,
		  "description":"Where we talk","reply":"discord.gg/example"}`, cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}

	for _, c := range h.commands(t, cookie) {
		if c.Name != "discord" {
			continue
		}
		if c.Kind != "custom" {
			t.Errorf("kind = %q, want custom", c.Kind)
		}
		if c.Usage != "!discord" || c.Summary != "Where we talk" {
			t.Errorf("row = %+v", c)
		}
		return
	}
	t.Fatal("the command was not in the list")
}

// Including one that runs console commands, with the tier reported so the panel
// can show what is being handed out.
func TestACustomCommandReportsWhatItsLinesAre(t *testing.T) {
	h, cookie := chatHarness(t)

	rec := h.do(t, h.request(t, http.MethodPut, "/api/chat/commands/starter",
		`{"enabled":true,"audience":"admins","cooldownSeconds":3600,
		  "commands":["give {entityid} resourceWood 500","killall"]}`, cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}

	for _, c := range h.commands(t, cookie) {
		if c.Name != "starter" {
			continue
		}
		if len(c.Commands) != 2 {
			t.Fatalf("lines = %+v", c.Commands)
		}
		if c.Commands[0].Tier != "mutating" {
			t.Errorf("give is tier %q, want mutating", c.Commands[0].Tier)
		}
		if c.Commands[1].Tier != "destructive" {
			t.Errorf("killall is tier %q, want destructive", c.Commands[1].Tier)
		}
		if c.Tier != "destructive" {
			t.Errorf("the command is tier %q; the strongest line decides", c.Tier)
		}
		return
	}
	t.Fatal("the command was not in the list")
}

// The operator's own switch decides whether a destructive line will run, and
// the panel says so rather than letting them find out from a player.
func TestADestructiveLineIsMarkedBlockedWhenTheOperatorTurnedItOff(t *testing.T) {
	h, cookie := chatHarness(t)
	h.server.cfg.Panel.AllowDestructive = false

	rec := h.do(t, h.request(t, http.MethodPut, "/api/chat/commands/wipe",
		`{"enabled":true,"audience":"admins","cooldownSeconds":0,"commands":["killall"]}`, cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}

	for _, c := range h.commands(t, cookie) {
		if c.Name == "wipe" {
			if !c.Commands[0].Blocked {
				t.Error("the line is not marked blocked with the switch off")
			}
			return
		}
	}
	t.Fatal("the command was not in the list")
}

// A built-in's name is reserved: writing to it configures the built-in rather
// than replacing it, so nobody can shadow !kit with something else.
func TestABuiltinNameStaysTheBuiltin(t *testing.T) {
	h, cookie := chatHarness(t)

	rec := h.do(t, h.request(t, http.MethodPut, "/api/chat/commands/kit",
		`{"enabled":true,"audience":"everyone","cooldownSeconds":0,
		  "reply":"mine now","commands":["killall"]}`, cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}

	for _, c := range h.commands(t, cookie) {
		if c.Name != "kit" {
			continue
		}
		if c.Kind != "builtin" {
			t.Errorf("kind = %q, want builtin", c.Kind)
		}
		if len(c.Commands) != 0 || c.Reply != "" {
			t.Errorf("a built-in took on a body: %+v", c)
		}
		return
	}
	t.Fatal("kit was not in the list")
}

func TestACustomCommandCanBeDeletedAndABuiltinCannot(t *testing.T) {
	h, cookie := chatHarness(t)
	rec := h.do(t, h.request(t, http.MethodPut, "/api/chat/commands/discord",
		`{"enabled":true,"audience":"admins","cooldownSeconds":0,"reply":"hello"}`, cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}

	rec = h.do(t, h.request(t, http.MethodDelete, "/api/chat/commands/discord", "", cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("delete returned %d: %s", rec.Code, rec.Body.String())
	}
	rec = h.do(t, h.request(t, http.MethodDelete, "/api/chat/commands/discord", "", cookie))
	if rec.Code != http.StatusNotFound {
		t.Errorf("deleting twice returned %d, want 404", rec.Code)
	}

	rec = h.do(t, h.request(t, http.MethodDelete, "/api/chat/commands/kit", "", cookie))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("deleting a built-in returned %d, want 400", rec.Code)
	}
}

func TestCustomCommandRejections(t *testing.T) {
	tests := []struct {
		name string
		path string
		body string
	}{
		{
			// It has to be typeable in chat.
			name: "a name with a space",
			path: "/api/chat/commands/my%20command",
			body: `{"enabled":true,"audience":"admins","cooldownSeconds":0,"reply":"x"}`,
		},
		{
			name: "nothing to say and nothing to run",
			path: "/api/chat/commands/empty",
			body: `{"enabled":true,"audience":"admins","cooldownSeconds":0}`,
		},
		{
			// A second command smuggled onto one line would never appear in
			// what the admin reviewed.
			name: "two commands on one line",
			path: "/api/chat/commands/sneaky",
			body: `{"enabled":true,"audience":"admins","cooldownSeconds":0,` +
				`"commands":["say hello
shutdown"]}`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h, cookie := chatHarness(t)
			rec := h.do(t, h.request(t, http.MethodPut, tc.path, tc.body, cookie))
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestChatCommandRejections(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"unknown audience", `{"enabled":true,"audience":"mods","cooldownSeconds":0}`},
		{"negative cooldown", `{"enabled":true,"audience":"everyone","cooldownSeconds":-1}`},
		{"absurd cooldown", `{"enabled":true,"audience":"everyone","cooldownSeconds":999999999}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h, cookie := chatHarness(t)
			rec := h.do(t, h.request(t, http.MethodPut, "/api/chat/commands/day", tc.body, cookie))
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

/* ------------------------------------------------------------------ kits -- */

func TestKitRoundTrip(t *testing.T) {
	h, cookie := chatHarness(t)

	rec := h.do(t, h.request(t, http.MethodPut, "/api/chat/kits/starter",
		`{"items":[{"item":"resourceWood","count":500,"quality":0}]}`, cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}

	rec = h.do(t, h.request(t, http.MethodGet, "/api/chat/kits", "", cookie))
	kits := decode[struct {
		Kits []kitRow `json:"kits"`
	}](t, rec).Kits
	if len(kits) != 1 || kits[0].Name != "starter" || len(kits[0].Items) != 1 {
		t.Fatalf("kits = %+v", kits)
	}
	if kits[0].Items[0].Count != 500 {
		t.Errorf("count = %d, want 500", kits[0].Items[0].Count)
	}

	rec = h.do(t, h.request(t, http.MethodDelete, "/api/chat/kits/starter", "", cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("delete returned %d: %s", rec.Code, rec.Body.String())
	}
	rec = h.do(t, h.request(t, http.MethodDelete, "/api/chat/kits/starter", "", cookie))
	if rec.Code != http.StatusNotFound {
		t.Errorf("deleting twice returned %d, want 404", rec.Code)
	}
}

// A typo fails here, at the keyboard of the person who made it, rather than in
// front of the player who asked for the kit at three in the morning.
func TestAKitCannotHoldAnItemTheServerDoesNotHave(t *testing.T) {
	h, cookie := chatHarness(t)

	rec := h.do(t, h.request(t, http.MethodPut, "/api/chat/kits/typo",
		`{"items":[{"item":"resourceWoood","count":1}]}`, cookie))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	if got := decode[map[string]any](t, rec)["code"]; got != "UNKNOWN_ITEM" {
		t.Errorf("code = %v, want UNKNOWN_ITEM", got)
	}
}

func TestKitRejections(t *testing.T) {
	tests := []struct {
		name string
		path string
		body string
	}{
		{"no items", "/api/chat/kits/empty", `{"items":[]}`},
		{"a name with a space", "/api/chat/kits/my%20kit", `{"items":[{"item":"resourceWood"}]}`},
		{"a name that could be two arguments", "/api/chat/kits/a;b", `{"items":[{"item":"resourceWood"}]}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h, cookie := chatHarness(t)
			rec := h.do(t, h.request(t, http.MethodPut, tc.path, tc.body, cookie))
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

// Configuration is per server, like everything else the panel keeps: two worlds
// with two sets of players should not have to share a policy.
func TestChatCommandsAreScopedToTheirServer(t *testing.T) {
	h, cookie := chatHarness(t)

	rec := h.do(t, h.request(t, http.MethodPut, "/api/chat/commands/kit",
		`{"enabled":true,"audience":"everyone","cooldownSeconds":60}`, cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}

	other, err := h.store.ChatCommands(t.Context(), "somewhere-else")
	if err != nil {
		t.Fatal(err)
	}
	if len(other) != 0 {
		t.Errorf("another server sees %+v", other)
	}

	// And the count is right for the server it was written for.
	mine, err := h.store.ChatCommands(t.Context(), testServerID)
	if err != nil {
		t.Fatal(err)
	}
	if len(mine) != 1 {
		t.Errorf("this server sees %d rows, want 1", len(mine))
	}
}

// Kits are the exception, and deliberately so: a kit is a list of item names,
// and the items belong to the game rather than to one world.
func TestKitsAreSharedAcrossServers(t *testing.T) {
	h, cookie := chatHarness(t)
	rec := h.do(t, h.request(t, http.MethodPut, "/api/chat/kits/starter",
		`{"items":[{"item":"resourceWood","count":1}]}`, cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}

	kits, err := h.store.Kits(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(kits) != 1 {
		t.Fatalf("kits = %+v", kits)
	}
	// Nothing about the stored kit names a server.
	var _ *servers.Server = h.srv
}

package api

import (
	"net/http"
	"testing"

	"github.com/nomansheikh/7dtd-panel/internal/sdtd"
	"github.com/nomansheikh/7dtd-panel/internal/servers"
	"github.com/nomansheikh/7dtd-panel/internal/state"
)

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

// An operator cannot invent a command. The set is fixed in the chat package,
// and a row configuring something that does not exist would look like it did.
func TestAnUnknownChatCommandCannotBeConfigured(t *testing.T) {
	h, cookie := chatHarness(t)

	rec := h.do(t, h.request(t, http.MethodPut, "/api/chat/commands/shutdown",
		`{"enabled":true,"audience":"everyone","cooldownSeconds":0}`, cookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404: %s", rec.Code, rec.Body.String())
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

package api

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/nomansheikh/7dtd-panel/internal/sdtd"
	"github.com/nomansheikh/7dtd-panel/internal/state"
)

func playerHarness(t *testing.T, players []sdtd.Player) (*harness, *fakeGame, *http.Cookie) {
	t.Helper()
	h := newHarness(t, state.Snapshot{Status: state.StatusOnline})
	game := &fakeGame{players: players}
	h.srv.Client = game
	h.srv.Buffs = &fakeBuffs{buffs: []sdtd.Buff{
		{Name: "buffBrokenLeg", LocalizedName: "Broken Leg"},
		{Name: "buffInjuryDeepLaceration", LocalizedName: "Deep Laceration"},
	}}
	h.srv.Items = &fakeItems{items: []sdtd.Item{
		{Name: "resourceWood", LocalizedName: "Wood"},
	}}
	return h, game, h.login(t, "admin", testPassword)
}

// livePlayer mirrors a row captured from a real server.
func livePlayer() sdtd.Player {
	seen := time.Date(2026, 9, 11, 13, 37, 28, 0, time.UTC)
	return sdtd.Player{
		EntityID: 173, Name: "nullish",
		PlatformID: "Steam_76561198803325430",
		Online:     true, Ping: 0,
		Position:        sdtd.Position{X: -272.96875, Y: 61.09375, Z: 449},
		Level:           1,
		Health:          100,
		PlayTimeSeconds: 414,
		LastOnline:      &seen,
	}
}

func TestPlayersListing(t *testing.T) {
	h, _, cookie := playerHarness(t, []sdtd.Player{livePlayer()})

	rec := h.do(t, h.request(t, http.MethodGet, "/api/players", "", cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Players []playerRow `json:"players"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	got := body.Players[0]

	// Both halves of the merge have to survive to the client: level comes from
	// /api/player, playtime from the legacy endpoint.
	if got.Level != 1 {
		t.Errorf("level = %d, want 1", got.Level)
	}
	if got.PlayTimeSeconds != 414 {
		t.Errorf("playTimeSeconds = %d, want 414", got.PlayTimeSeconds)
	}
	// Position is fractional on a real server despite the spec calling it a
	// Vector3i, so it must not be rounded away in transit.
	if got.Position == nil || got.Position.X != -272.96875 {
		t.Errorf("position = %+v, want the exact float", got.Position)
	}
}

func TestOfflinePlayersOmitOnlineOnlyFields(t *testing.T) {
	offline := livePlayer()
	offline.Online = false
	offline.Position = sdtd.Position{}

	h, _, cookie := playerHarness(t, []sdtd.Player{offline})
	rec := h.do(t, h.request(t, http.MethodGet, "/api/players", "", cookie))

	var body struct {
		Players []playerRow `json:"players"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)

	// Reporting 0,0,0 would be a claim about where they are, which is unknown.
	if body.Players[0].Position != nil {
		t.Errorf("position = %+v, want omitted for an offline player", body.Players[0].Position)
	}
}

func TestPlayersSortedOnlineFirst(t *testing.T) {
	older := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)

	h, _, cookie := playerHarness(t, []sdtd.Player{
		{Name: "old", PlatformID: "Steam_1", LastOnline: &older},
		{Name: "recent", PlatformID: "Steam_2", LastOnline: &newer},
		{Name: "here", PlatformID: "Steam_3", Online: true},
	})
	rec := h.do(t, h.request(t, http.MethodGet, "/api/players", "", cookie))

	var body struct {
		Players []playerRow `json:"players"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)

	// The people you can act on now belong at the top, then most recently seen.
	want := []string{"here", "recent", "old"}
	for i, name := range want {
		if body.Players[i].Name != name {
			t.Errorf("row %d = %q, want %q", i, body.Players[i].Name, name)
		}
	}
}

func TestPlayerActionsBuildTheRightCommands(t *testing.T) {
	tests := []struct {
		name string
		path string
		body string
		want string
	}{
		{"teleport to coords", "/api/players/173/teleport", `{"x":10,"y":-1,"z":20}`, "teleportplayer 173 10 -1 20"},
		{"teleport to player", "/api/players/173/teleport", `{"toEntityId":174}`, "teleportplayer 173 174"},
		{"give", "/api/players/173/give", `{"item":"resourceWood","count":5,"quality":0}`, "give 173 resourceWood 5"},
		{"give with quality", "/api/players/173/give", `{"item":"resourceWood","count":1,"quality":6}`, "give 173 resourceWood 1 6"},
		{"kill", "/api/players/173/kill", `{}`, "kill 173"},
		{"kick", "/api/players/173/kick", `{"reason":"afk"}`, "kick 173 afk"},
		{"kick without reason", "/api/players/173/kick", `{"reason":""}`, "kick 173"},
		{"buff", "/api/players/173/buff", `{"buff":"buffBrokenLeg","remove":false}`, "buffplayer 173 buffBrokenLeg"},
		{"debuff", "/api/players/173/buff", `{"buff":"buffBrokenLeg","remove":true}`, "debuffplayer 173 buffBrokenLeg"},
		{"xp", "/api/players/173/xp", `{"amount":500}`, "givexp 173 500"},
		{
			// Ban takes a platform id, because it must work after they leave.
			name: "ban", path: "/api/players/ban",
			body: `{"platformId":"Steam_76561198803325430","duration":3,"unit":"days","reason":"griefing"}`,
			want: "ban add Steam_76561198803325430 3 days griefing",
		},
		{
			name: "unban", path: "/api/players/unban",
			body: `{"platformId":"Steam_76561198803325430","duration":1,"unit":"days","reason":""}`,
			want: "ban remove Steam_76561198803325430",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, game, cookie := playerHarness(t, []sdtd.Player{livePlayer()})
			rec := h.do(t, h.request(t, http.MethodPost, tt.path, tt.body, cookie))
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
			}
			if len(game.executed) != 1 || game.executed[0] != tt.want {
				t.Errorf("executed %v, want [%s]", game.executed, tt.want)
			}
		})
	}
}

func TestPlayerActionsRejectBadInput(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		body     string
		wantCode string
	}{
		{"unknown item", "/api/players/173/give", `{"item":"notAThing","count":1}`, "UNKNOWN_ITEM"},
		{"injection via item name", "/api/players/173/give", `{"item":"wood shutdown","count":1}`, "UNKNOWN_ITEM"},
		{"zero count", "/api/players/173/give", `{"item":"resourceWood","count":0}`, "INVALID_ARGUMENT"},
		{"kick reason with a newline", "/api/players/173/kick", `{"reason":"bye\nshutdown"}`, "INVALID_ARGUMENT"},
		{"kick reason with a quote", "/api/players/173/kick", `{"reason":"said \"no\""}`, "INVALID_ARGUMENT"},
		{"teleport with neither target", "/api/players/173/teleport", `{}`, "INVALID_ARGUMENT"},
		{"buff name with a space", "/api/players/173/buff", `{"buff":"buff shutdown"}`, "INVALID_ARGUMENT"},
		{"zero xp", "/api/players/173/xp", `{"amount":0}`, "INVALID_ARGUMENT"},
		{"ban with a bare name", "/api/players/ban", `{"platformId":"nullish","duration":1,"unit":"days"}`, "INVALID_ARGUMENT"},
		{"ban with a bad unit", "/api/players/ban", `{"platformId":"Steam_1","duration":1,"unit":"fortnights"}`, "INVALID_ARGUMENT"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, game, cookie := playerHarness(t, []sdtd.Player{livePlayer()})
			rec := h.do(t, h.request(t, http.MethodPost, tt.path, tt.body, cookie))

			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400 (%s)", rec.Code, rec.Body.String())
			}
			if got := decode[map[string]any](t, rec)["code"]; got != tt.wantCode {
				t.Errorf("code = %v, want %v", got, tt.wantCode)
			}
			if len(game.executed) != 0 {
				t.Errorf("a rejected action still reached the game server: %v", game.executed)
			}
		})
	}
}

func TestNonNumericEntityIDIsRejected(t *testing.T) {
	h, game, cookie := playerHarness(t, nil)

	req := h.request(t, http.MethodPost, "/api/players/173/kill", `{}`, cookie)
	req.URL.Path = "/api/servers/" + testServerID + "/players/notanumber/kill"

	rec := h.do(t, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	if len(game.executed) != 0 {
		t.Error("a bad entity id still reached the game server")
	}
}

func TestPlayerEndpointsRequireAuth(t *testing.T) {
	h := newHarness(t, state.Snapshot{})
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/players", ""},
		{http.MethodPost, "/api/players/1/kill", `{}`},
		{http.MethodPost, "/api/players/ban", `{}`},
	} {
		t.Run(tc.path, func(t *testing.T) {
			rec := h.do(t, h.request(t, tc.method, tc.path, tc.body, nil))
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401", rec.Code)
			}
		})
	}
}

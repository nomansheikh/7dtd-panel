package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/nomansheikh/7dtd-panel/internal/sdtd"
	"github.com/nomansheikh/7dtd-panel/internal/state"
)

type fakeEntities struct {
	classes  []sdtd.EntityClass
	err      error
	gotLimit int
}

func (f *fakeEntities) Search(_ context.Context, _ string, spawnableOnly bool, limit int) ([]sdtd.EntityClass, int, error) {
	f.gotLimit = limit
	if f.err != nil {
		return nil, 0, f.err
	}
	var out []sdtd.EntityClass
	for _, c := range f.classes {
		if spawnableOnly && c.ManualSpawnType == "None" {
			continue
		}
		out = append(out, c)
	}
	return out, len(out), nil
}

func (f *fakeEntities) Lookup(_ context.Context, name string) (sdtd.EntityClass, bool, error) {
	if f.err != nil {
		return sdtd.EntityClass{}, false, f.err
	}
	for _, c := range f.classes {
		if c.Name == name {
			return c, true, nil
		}
	}
	return sdtd.EntityClass{}, false, nil
}

type fakeItems struct{ items []sdtd.Item }

func (f *fakeItems) Search(_ context.Context, _ string, _ bool, _ int) ([]sdtd.Item, int, error) {
	return f.items, len(f.items), nil
}

func (f *fakeItems) Has(_ context.Context, name string) (bool, error) {
	for _, i := range f.items {
		if i.Name == name {
			return true, nil
		}
	}
	return false, nil
}

type fakeBuffs struct{ buffs []sdtd.Buff }

func (f *fakeBuffs) Search(_ context.Context, _ string, _ int) ([]sdtd.Buff, int, error) {
	return f.buffs, len(f.buffs), nil
}

func (f *fakeBuffs) Has(_ context.Context, name string) (bool, error) {
	for _, b := range f.buffs {
		if b.Name == name {
			return true, nil
		}
	}
	return false, nil
}

func worldHarness(t *testing.T) (*harness, *fakeGame, *http.Cookie) {
	t.Helper()
	h := newHarness(t, state.Snapshot{Status: state.StatusOnline})
	game := &fakeGame{}
	h.srv.Client = game
	h.srv.Entities = &fakeEntities{classes: []sdtd.EntityClass{
		{Name: "zombieBoe", ID: 1234, ManualSpawnType: "Spawn"},
		{Name: "playerMale", ID: 2001454542, ManualSpawnType: "None"},
	}}
	h.srv.Items = &fakeItems{items: []sdtd.Item{
		{Name: "meleeToolStoneAxe", LocalizedName: "Stone Axe"},
	}}
	return h, game, h.login(t, "admin", testPassword)
}

func TestWorldActionsBuildTheRightCommands(t *testing.T) {
	tests := []struct {
		name string
		path string
		body string
		want string
	}{
		{"set time", "/api/world/time", `{"day":7,"hour":21,"minute":30}`, "settime 7 21 30"},
		{"weather rain", "/api/world/weather", `{"setting":"Rain","value":0.5}`, "weather Rain 0.5"},
		{"weather defaults", "/api/world/weather", `{"setting":"","value":null,"defaults":true}`, "weather Defaults"},
		// Quoted, or the game broadcasts only the first word.
		{"say", "/api/world/say", `{"message":"restarting soon"}`, `say "restarting soon"`},
		{"air drop", "/api/world/airdrop", "{}", "spawnairdrop"},
		{"wandering horde", "/api/world/horde", ``, "spawnwandering"},
		{
			// The command takes the class name, not the id from
			// /api/entityclass: that id is a hash and the server rejects it.
			// The lookup still matters, to prove the class exists and is
			// spawnable before anything is sent.
			name: "spawn uses the class name, not its id",
			path: "/api/world/spawn",
			body: `{"entityClass":"zombieBoe","x":10,"y":-1,"z":20,"count":3}`,
			want: "spawnentityat zombieBoe 10 -1 20 3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, game, cookie := worldHarness(t)
			rec := h.do(t, h.request(t, http.MethodPost, tt.path, tt.body, cookie))
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
			}
			if len(game.executed) != 1 || game.executed[0] != tt.want {
				t.Errorf("executed %v, want [%s]", game.executed, tt.want)
			}
			// The command is echoed so an operator can see exactly what ran.
			var body actionResponse
			_ = json.Unmarshal(rec.Body.Bytes(), &body)
			if body.Command != tt.want {
				t.Errorf("echoed command = %q, want %q", body.Command, tt.want)
			}
		})
	}
}

func TestWorldActionsRejectBadArguments(t *testing.T) {
	tests := []struct {
		name string
		path string
		body string
	}{
		{"hour out of range", "/api/world/time", `{"day":1,"hour":99,"minute":0}`},
		{"day below one", "/api/world/time", `{"day":0,"hour":0,"minute":0}`},
		{"unknown weather setting", "/api/world/weather", `{"setting":"Sunshine","value":1}`},
		{"rain above one", "/api/world/weather", `{"setting":"Rain","value":5}`},
		{"weather without a value", "/api/world/weather", `{"setting":"Rain"}`},
		{"empty message", "/api/world/say", `{"message":"   "}`},
		{"message with a newline", "/api/world/say", `{"message":"hi\nshutdown"}`},
		{"message with a quote", "/api/world/say", `{"message":"say \"hi\""}`},
		{"spawn count too high", "/api/world/spawn", `{"entityClass":"zombieBoe","x":0,"y":0,"z":0,"count":500}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, game, cookie := worldHarness(t)
			rec := h.do(t, h.request(t, http.MethodPost, tt.path, tt.body, cookie))
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400 (body %s)", rec.Code, rec.Body.String())
			}
			if len(game.executed) != 0 {
				t.Errorf("a rejected request still reached the game server: %v", game.executed)
			}
		})
	}
}

func TestSpawnRejectsUnknownAndUnspawnableClasses(t *testing.T) {
	tests := []struct {
		name     string
		class    string
		wantCode string
	}{
		{"unknown class", "notAnEntity", "UNKNOWN_ENTITY"},
		// Offering these would only produce a command that always fails.
		{"class the game will not spawn", "playerMale", "NOT_SPAWNABLE"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, game, cookie := worldHarness(t)
			rec := h.do(t, h.request(t, http.MethodPost, "/api/world/spawn",
				`{"entityClass":"`+tt.class+`","x":0,"y":0,"z":0,"count":1}`, cookie))

			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", rec.Code)
			}
			if got := decode[map[string]any](t, rec)["code"]; got != tt.wantCode {
				t.Errorf("code = %v, want %v", got, tt.wantCode)
			}
			if len(game.executed) != 0 {
				t.Error("a rejected spawn still reached the game server")
			}
		})
	}
}

func TestWorldActionsAreRecordedInHistory(t *testing.T) {
	h, _, cookie := worldHarness(t)
	h.do(t, h.request(t, http.MethodPost, "/api/world/time", `{"day":3,"hour":12,"minute":0}`, cookie))

	rec := h.do(t, h.request(t, http.MethodGet, "/api/console/history", "", cookie))
	var hist struct {
		History []historyEntry `json:"history"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &hist)

	// Everything an operator did belongs in one place, whichever surface they
	// used to do it.
	if len(hist.History) != 1 || hist.History[0].Command != "settime 3 12 0" {
		t.Errorf("history = %+v, want the world action recorded", hist.History)
	}
}

func TestWorldActionsRequireAuth(t *testing.T) {
	h := newHarness(t, state.Snapshot{})
	for _, path := range []string{
		"/api/world/time", "/api/world/weather", "/api/world/spawn",
		"/api/world/horde", "/api/world/say",
	} {
		t.Run(path, func(t *testing.T) {
			rec := h.do(t, h.request(t, http.MethodPost, path, `{}`, nil))
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401", rec.Code)
			}
		})
	}
}

func TestEntityPickerHidesUnspawnableByDefault(t *testing.T) {
	h, _, cookie := worldHarness(t)

	rec := h.do(t, h.request(t, http.MethodGet, "/api/entities?q=", "", cookie))
	var body struct {
		Entities []sdtd.EntityClass `json:"entities"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)

	for _, e := range body.Entities {
		if e.ManualSpawnType == "None" {
			t.Errorf("%s cannot be spawned but was offered in the picker", e.Name)
		}
	}
}

// The storm command answers an unknown biome with silence, so the panel has to
// refuse a name the server did not just report rather than sending it.
func TestStormRejectsABiomeTheWorldDoesNotHave(t *testing.T) {
	h, game, cookie := worldHarness(t)
	game.weather = sdtd.Weather{Biomes: []sdtd.BiomeWeather{{Biome: "pine_forest"}}}

	rec := h.do(t, h.request(t, http.MethodPost, "/api/world/weather",
		`{"storm":true,"stormHours":2,"biome":"not_a_biome"}`, cookie))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if len(game.executed) != 0 {
		t.Errorf("nothing should have reached the server, got %v", game.executed)
	}

	rec = h.do(t, h.request(t, http.MethodPost, "/api/world/weather",
		`{"storm":true,"stormHours":2,"biome":"pine_forest"}`, cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if len(game.executed) != 1 || game.executed[0] != "weather Storm 2 pine_forest" {
		t.Errorf("executed %v", game.executed)
	}
}

func TestEntitiesLimitReachesTheCatalogue(t *testing.T) {
	for _, tc := range []struct {
		name  string
		query string
		want  int
	}{
		{"asked for", "?q=&limit=600", 600},
		{"absent", "?q=", 0},
		{"not a number", "?q=&limit=all", 0},
		{"negative", "?q=&limit=-5", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, _, cookie := worldHarness(t)
			entities := &fakeEntities{classes: []sdtd.EntityClass{
				{Name: "zombieArlene", ManualSpawnType: "Menu"},
			}}
			h.srv.Entities = entities

			rec := h.do(t, h.request(t, http.MethodGet, "/api/entities"+tc.query, "", cookie))
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			if entities.gotLimit != tc.want {
				t.Fatalf("limit reaching the catalogue = %d, want %d", entities.gotLimit, tc.want)
			}
		})
	}
}

func TestServerVitalsAreServed(t *testing.T) {
	h, game, cookie := worldHarness(t)
	game.health = sdtd.Health{
		FPS: 19.99, RSSMB: 2595.4, Chunks: 253, Players: 2,
		GameVersion: "V 3.2.0 (b10)",
		Mods:        []sdtd.Mod{{Name: "TFP_Harmony", Version: "1.1.0.4"}},
	}

	rec := h.do(t, h.request(t, http.MethodGet, "/api/vitals", "", cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body sdtd.Health
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.FPS != 19.99 || body.Chunks != 253 || body.GameVersion != "V 3.2.0 (b10)" {
		t.Errorf("health = %+v", body)
	}
	if len(body.Mods) != 1 {
		t.Errorf("mods = %d, want 1", len(body.Mods))
	}
}

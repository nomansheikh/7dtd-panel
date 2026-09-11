package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/nomansheikh/7dtd-panel/internal/sdtd"
	"github.com/nomansheikh/7dtd-panel/internal/state"
)

func pref(name, kind, value, def string) sdtd.TypedValue {
	return sdtd.TypedValue{
		Name: name, Type: kind,
		Raw:     json.RawMessage(value),
		Default: json.RawMessage(def),
	}
}

func choice(index int, value, label string) sdtd.SandboxChoice {
	return sdtd.SandboxChoice{Index: index, Raw: json.RawMessage(value), Label: label}
}

// settingsHarness wires a server whose preference list and sandbox options
// overlap the way a real one does.
func settingsHarness(t *testing.T) (*harness, *fakeGame, *http.Cookie) {
	t.Helper()
	h := newHarness(t, state.Snapshot{Status: state.StatusOnline})
	h.game.prefs = sdtd.NewValueSet([]sdtd.TypedValue{
		pref("AirDropFrequency", "int", "3", "3"),
		pref("XPMultiplier", "int", "100", "100"),
		pref("ServerName", "string", `"Test"`, `"My Game Host"`),
		pref("BloodMoonWarning", "int", "1", "8"),
		pref("ServerMaxPlayerCount", "int", "8", "8"),
		pref("OptionsGfxFOV", "int", "65", "65"),
	})
	h.game.sandbox = sdtd.SandboxSettings{
		Code: "AAAJABJ",
		Options: []sdtd.SandboxOption{
			{
				Key: "AirDropFrequency", Type: "Int",
				ActiveValue:  choice(2, "3", "3 Days"),
				DefaultValue: choice(2, "3", "3 Days"),
				ValueSet: []sdtd.SandboxChoice{
					choice(0, "0", "Never"), choice(1, "1", "1 Day"), choice(2, "3", "3 Days"),
				},
				Details: &sdtd.SandboxDetails{
					CategoryName: "World", LocalizedName: "Air Drop Frequency",
					Description: "How often a supply crate is dropped.",
				},
			},
			{
				// The sandbox screen holds this as a fraction while the
				// preference holds a percentage.
				Key: "XPMultiplier", Type: "Float",
				ActiveValue:  choice(1, "1", "100%"),
				DefaultValue: choice(1, "1", "100%"),
				ValueSet: []sdtd.SandboxChoice{
					choice(0, "0.5", "50%"), choice(1, "1", "100%"), choice(2, "2", "200%"),
				},
				Details: &sdtd.SandboxDetails{
					CategoryName: "General", LocalizedName: "XP Multiplier",
					Description: "Experience gained.",
				},
			},
			{
				// The preference's default is on a different scale from the
				// choices, which is real: BloodMoonWarning reports 8.
				Key: "BloodMoonWarning", Type: "Int",
				ActiveValue:  choice(0, "1", "Morning"),
				DefaultValue: choice(0, "1", "Morning"),
				ValueSet: []sdtd.SandboxChoice{
					choice(0, "1", "Morning"), choice(1, "2", "Midday"),
				},
				Details: &sdtd.SandboxDetails{
					CategoryName: "World", LocalizedName: "Blood Moon Warning",
					Description: "When the warning appears.",
				},
			},
			{
				// Not a preference, so it cannot be changed at runtime.
				Key: "RangedDamage", Type: "Float",
				ActiveValue:  choice(9, "1.5", "150%"),
				DefaultValue: choice(7, "1", "100%"),
				ValueSet:     []sdtd.SandboxChoice{choice(7, "1", "100%"), choice(9, "1.5", "150%")},
				Details: &sdtd.SandboxDetails{
					CategoryName: "General", LocalizedName: "Ranged Damage",
					Description: "Damage dealt with ranged weapons.",
				},
			},
		},
	}
	return h, h.game, h.login(t, "admin", testPassword)
}

type settingChoiceJSON struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type settingJSON struct {
	Name           string              `json:"name"`
	Label          string              `json:"label"`
	Description    string              `json:"description"`
	Type           string              `json:"type"`
	Value          any                 `json:"value"`
	ValueLabel     string              `json:"valueLabel"`
	Changed        bool                `json:"changed"`
	DefaultLabel   string              `json:"defaultLabel"`
	Editable       bool                `json:"editable"`
	ReadOnlyReason string              `json:"readOnlyReason"`
	Choices        []settingChoiceJSON `json:"choices"`
}

type settingsResponse struct {
	SandboxCode string `json:"sandboxCode"`
	Sections    []struct {
		ID     string `json:"id"`
		Total  int    `json:"total"`
		Groups []struct {
			Name     string        `json:"name"`
			Settings []settingJSON `json:"settings"`
		} `json:"groups"`
	} `json:"sections"`
}

func (r settingsResponse) find(t *testing.T, section, name string) settingJSON {
	t.Helper()
	for _, s := range r.Sections {
		if s.ID != section {
			continue
		}
		for _, g := range s.Groups {
			for _, row := range g.Settings {
				if row.Name == name {
					return row
				}
			}
		}
	}
	t.Fatalf("no setting %q in section %q", name, section)
	panic("unreachable")
}

// The whole point of the page: a setting the game enumerates arrives as a list
// of choices in the game's own words, not as a number to be guessed.
func TestSettingsTurnEnumeratedOptionsIntoChoices(t *testing.T) {
	h, _, cookie := settingsHarness(t)

	rec := h.do(t, h.request(t, http.MethodGet, "/api/settings", "", cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	got := decode[settingsResponse](t, rec)

	if got.SandboxCode != "AAAJABJ" {
		t.Errorf("sandboxCode = %q", got.SandboxCode)
	}

	drop := got.find(t, "world", "AirDropFrequency")
	if !drop.Editable {
		t.Error("AirDropFrequency should be editable: it is also a preference")
	}
	if drop.Label != "Air Drop Frequency" {
		t.Errorf("label = %q, want the game's own wording", drop.Label)
	}
	if drop.Description == "" {
		t.Error("the game's description should be passed through")
	}
	if drop.ValueLabel != "3 Days" {
		t.Errorf("valueLabel = %q, want %q", drop.ValueLabel, "3 Days")
	}
	if len(drop.Choices) != 3 || drop.Choices[0].Label != "Never" {
		t.Errorf("choices = %+v", drop.Choices)
	}
}

// /api/gameprefs reports what the server started with. A page built on it
// alone shows an operator a value the server stopped using, and snaps their
// change back the moment it refetches.
func TestSettingsPreferTheLiveConsoleValue(t *testing.T) {
	h, game, cookie := settingsHarness(t)
	game.live = map[string]string{
		"AirDropFrequency":     "1",
		"ServerName":           "Renamed",
		"ServerMaxPlayerCount": "16",
	}

	rec := h.do(t, h.request(t, http.MethodGet, "/api/settings", "", cookie))
	got := decode[settingsResponse](t, rec)

	drop := got.find(t, "world", "AirDropFrequency")
	if drop.Value != float64(1) {
		t.Errorf("value = %v, want the console's 1 rather than the REST reply's 3", drop.Value)
	}
	if drop.ValueLabel != "1 Day" {
		t.Errorf("valueLabel = %q, want %q", drop.ValueLabel, "1 Day")
	}
	if !drop.Changed {
		t.Error("1 differs from the default of 3; the row should be flagged")
	}
	if name := got.find(t, "server", "ServerName"); name.Value != "Renamed" {
		t.Errorf("ServerName = %v", name.Value)
	}
	if max := got.find(t, "server", "ServerMaxPlayerCount"); max.Value != float64(16) {
		t.Errorf("ServerMaxPlayerCount = %v", max.Value)
	}
}

// A preference the console does not report, or reports in a form that does not
// match its declared type, keeps the value the REST reply gave.
func TestSettingsKeepTheRestValueWhenTheConsoleDoesNot(t *testing.T) {
	h, game, cookie := settingsHarness(t)
	game.live = map[string]string{"ServerMaxPlayerCount": "not a number"}

	rec := h.do(t, h.request(t, http.MethodGet, "/api/settings", "", cookie))
	got := decode[settingsResponse](t, rec)

	if max := got.find(t, "server", "ServerMaxPlayerCount"); max.Value != float64(8) {
		t.Errorf("value = %v, want the REST reply's 8", max.Value)
	}
	// XPMultiplier is absent from the console reply entirely.
	if xp := got.find(t, "world", "XPMultiplier"); xp.Value != float64(100) {
		t.Errorf("XPMultiplier = %v", xp.Value)
	}
}

// The two sides disagree on scale for the percentage settings. Reading the
// sandbox value straight through would write 1 where the server wants 100.
func TestSettingsRescalePercentageChoices(t *testing.T) {
	h, _, cookie := settingsHarness(t)

	rec := h.do(t, h.request(t, http.MethodGet, "/api/settings", "", cookie))
	got := decode[settingsResponse](t, rec)

	xp := got.find(t, "world", "XPMultiplier")
	want := []string{"50", "100", "200"}
	if len(xp.Choices) != len(want) {
		t.Fatalf("choices = %+v", xp.Choices)
	}
	for i, w := range want {
		if xp.Choices[i].Value != w {
			t.Errorf("choice %d value = %q, want %q", i, xp.Choices[i].Value, w)
		}
	}
	if xp.ValueLabel != "100%" {
		t.Errorf("valueLabel = %q, want %q", xp.ValueLabel, "100%")
	}
}

// The preference's own default is not always on the same scale as the
// choices. Showing "Default 8" beside a picker offering Morning and Midday is
// worse than showing nothing, and flagging the row as changed is simply wrong.
func TestSettingsPreferTheSandboxDefaultWhenScalesDisagree(t *testing.T) {
	h, _, cookie := settingsHarness(t)

	rec := h.do(t, h.request(t, http.MethodGet, "/api/settings", "", cookie))
	got := decode[settingsResponse](t, rec)

	warning := got.find(t, "world", "BloodMoonWarning")
	if warning.DefaultLabel != "Morning" {
		t.Errorf("defaultLabel = %q, want %q", warning.DefaultLabel, "Morning")
	}
	if warning.ValueLabel != "Morning" {
		t.Errorf("valueLabel = %q", warning.ValueLabel)
	}
	if warning.Changed {
		t.Error("the value matches the sandbox default; it has not been changed")
	}
}

// A sandbox option with no matching preference cannot be changed while the
// server runs, and the page has to say so rather than offer a dead control.
func TestSettingsMarkWorldOnlyOptionsReadOnly(t *testing.T) {
	h, _, cookie := settingsHarness(t)

	rec := h.do(t, h.request(t, http.MethodGet, "/api/settings", "", cookie))
	got := decode[settingsResponse](t, rec)

	ranged := got.find(t, "world", "RangedDamage")
	if ranged.Editable {
		t.Error("RangedDamage is not a preference and cannot be set at runtime")
	}
	if ranged.ReadOnlyReason == "" {
		t.Error("a disabled control needs an explanation")
	}
	if ranged.ValueLabel != "150%" {
		t.Errorf("valueLabel = %q", ranged.ValueLabel)
	}
}

// Client graphics options arrive in the same list and would otherwise bury the
// settings that matter.
func TestSettingsSeparateClientPreferences(t *testing.T) {
	h, _, cookie := settingsHarness(t)

	rec := h.do(t, h.request(t, http.MethodGet, "/api/settings", "", cookie))
	got := decode[settingsResponse](t, rec)

	fov := got.find(t, "client", "OptionsGfxFOV")
	if fov.Editable {
		t.Error("a client preference should not be presented as changeable")
	}

	for _, s := range got.Sections {
		if s.ID != "server" {
			continue
		}
		for _, g := range s.Groups {
			for _, row := range g.Settings {
				if row.Name == "OptionsGfxFOV" || row.Name == "AirDropFrequency" {
					t.Errorf("%s should not be in the server section", row.Name)
				}
			}
		}
	}
	got.find(t, "server", "ServerName")
}

// The page is worth showing even when the descriptions cannot be fetched.
func TestSettingsSurviveTheSandboxCallFailing(t *testing.T) {
	h, game, cookie := settingsHarness(t)
	game.sandbox = sdtd.SandboxSettings{}
	game.sandboxErr = errors.New("game server unreachable")

	rec := h.do(t, h.request(t, http.MethodGet, "/api/settings", "", cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	got := decode[settingsResponse](t, rec)
	// Without the sandbox list it is a plain preference again, but it is there.
	got.find(t, "server", "AirDropFrequency")
}

func TestUpdateSettingReadsTheValueBack(t *testing.T) {
	h, game, cookie := settingsHarness(t)
	// /api/gameprefs does not reflect runtime changes, so the console is the
	// only honest source for what actually took effect.
	game.readBack = map[string]string{"AirDropFrequency": "7"}

	rec := h.do(t, h.request(t, http.MethodPut,
		"/api/settings/AirDropFrequency", `{"value":"5"}`, cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	got := decode[struct {
		Value     string `json:"value"`
		Command   string `json:"command"`
		Persisted bool   `json:"persisted"`
	}](t, rec)

	if got.Value != "7" {
		t.Errorf("value = %q, want the server's answer %q, not the request", got.Value, "7")
	}
	if got.Command != "setgamepref AirDropFrequency 5" {
		t.Errorf("command = %q", got.Command)
	}
	if got.Persisted {
		t.Error("persisted should be false: the game never writes this back to its config")
	}
	if len(game.executed) == 0 || game.executed[0] != "setgamepref AirDropFrequency 5" {
		t.Errorf("executed %v", game.executed)
	}
}

// The console answers a rejected value with text and a 200, so the reply has
// to be read rather than the status alone.
func TestUpdateSettingReportsAConsoleRejection(t *testing.T) {
	h, game, cookie := settingsHarness(t)
	game.result = sdtd.CommandResult{
		Command: "setgamepref",
		Result:  "Error parsing parameter: AirDropFrequency\n",
	}

	rec := h.do(t, h.request(t, http.MethodPut,
		"/api/settings/AirDropFrequency", `{"value":"5"}`, cookie))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateSettingRejections(t *testing.T) {
	tests := []struct {
		name    string
		setting string
		body    string
		code    string
	}{
		{"unknown setting", "NoSuchSetting", `{"value":"1"}`, "UNKNOWN_SETTING"},
		{"client preference", "OptionsGfxFOV", `{"value":"90"}`, "CLIENT_SETTING"},
		{"not a number", "AirDropFrequency", `{"value":"often"}`, "INVALID_VALUE"},
		{"string with a space", "ServerName", `{"value":"my server"}`, "INVALID_VALUE"},
		{"empty string", "ServerName", `{"value":""}`, "INVALID_VALUE"},
		// A newline would append a second console command.
		{"newline in the value", "ServerName", `{"value":"a\nkickall"}`, "INVALID_VALUE"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, game, cookie := settingsHarness(t)
			rec := h.do(t, h.request(t, http.MethodPut,
				"/api/settings/"+tt.setting, tt.body, cookie))
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
			}
			got := decode[struct {
				Code string `json:"code"`
			}](t, rec)
			if got.Code != tt.code {
				t.Errorf("code = %q, want %q", got.Code, tt.code)
			}
			if len(game.executed) != 0 {
				t.Errorf("nothing should have reached the server, got %v", game.executed)
			}
		})
	}
}

// setgamepref accepts the startup-only preferences and answers "set to x", but
// getgamepref then reports nothing for them and the server carries on with the
// value it read at boot. A control that silently does nothing is worse than no
// control.
func TestSettingsRefuseWhatTheConsoleWillNotReportBack(t *testing.T) {
	h, game, cookie := settingsHarness(t)
	// The console reports everything except ServerMaxPlayerCount.
	game.live = map[string]string{
		"AirDropFrequency": "3", "XPMultiplier": "100",
		"ServerName": "Test", "BloodMoonWarning": "1",
	}

	rec := h.do(t, h.request(t, http.MethodGet, "/api/settings", "", cookie))
	got := decode[settingsResponse](t, rec)

	max := got.find(t, "server", "ServerMaxPlayerCount")
	if max.Editable {
		t.Error("a preference the console will not report back should not be editable")
	}
	if max.ReadOnlyReason == "" {
		t.Error("a disabled control needs an explanation")
	}
	if name := got.find(t, "server", "ServerName"); !name.Editable {
		t.Error("ServerName is reported by the console and should be editable")
	}

	rec = h.do(t, h.request(t, http.MethodPut,
		"/api/settings/ServerMaxPlayerCount", `{"value":"16"}`, cookie))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if len(game.executed) != 0 {
		t.Errorf("no command should have been run, got %v", game.executed)
	}
}

// A console that cannot be read should not grey out the whole page; the write
// will report its own failure.
func TestSettingsStayEditableWhenTheConsoleIsUnreadable(t *testing.T) {
	h, game, cookie := settingsHarness(t)
	game.liveErr = errors.New("game server unreachable")

	rec := h.do(t, h.request(t, http.MethodGet, "/api/settings", "", cookie))
	got := decode[settingsResponse](t, rec)
	if !got.find(t, "server", "ServerName").Editable {
		t.Error("settings should stay editable when the live read fails")
	}
}

func TestSettingsRequireAuth(t *testing.T) {
	h, _, _ := settingsHarness(t)
	for _, r := range []*http.Request{
		h.request(t, http.MethodGet, "/api/settings", "", nil),
		h.request(t, http.MethodPut, "/api/settings/AirDropFrequency", `{"value":"5"}`, nil),
	} {
		if rec := h.do(t, r); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s = %d, want 401", r.Method, r.URL.Path, rec.Code)
		}
	}
}

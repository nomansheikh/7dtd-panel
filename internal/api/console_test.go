package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/nomansheikh/7dtd-panel/internal/events"
	"github.com/nomansheikh/7dtd-panel/internal/sdtd"
	"github.com/nomansheikh/7dtd-panel/internal/state"
)

// fakeGame records what was executed and returns a scripted outcome.
type fakeGame struct {
	executed   []string
	result     sdtd.CommandResult
	err        error
	players    []sdtd.Player
	playersErr error
	prefs      sdtd.ValueSet
	prefsErr   error
	// readBack lets a test simulate the server reporting a different value
	// than the one that was written.
	readBack      map[string]string
	lastPrefValue string
	sandbox       sdtd.SandboxSettings
	sandboxErr    error
	// live is what the console reports, which is not always what
	// /api/gameprefs reports.
	live       map[string]string
	liveErr    error
	weather    sdtd.Weather
	weatherErr error
	health     sdtd.Health
	healthErr  error
	icon       []byte
	iconErr    error
	iconFor    string
}

func (f *fakeGame) Players(context.Context) ([]sdtd.Player, error) {
	return f.players, f.playersErr
}

func (f *fakeGame) GamePrefs(context.Context) (sdtd.ValueSet, error) {
	return f.prefs, f.prefsErr
}

func (f *fakeGame) CurrentWeather(context.Context) (sdtd.Weather, error) {
	return f.weather, f.weatherErr
}

func (f *fakeGame) ServerHealth(context.Context) (sdtd.Health, error) {
	return f.health, f.healthErr
}

func (f *fakeGame) ItemIcon(_ context.Context, name, _ string) ([]byte, error) {
	if f.iconErr != nil {
		return nil, f.iconErr
	}
	f.iconFor = name
	return f.icon, nil
}

func (f *fakeGame) GamePrefsLive(context.Context) (map[string]string, error) {
	return f.live, f.liveErr
}

func (f *fakeGame) SandboxSettings(context.Context) (sdtd.SandboxSettings, error) {
	return f.sandbox, f.sandboxErr
}

func (f *fakeGame) ReadGamePref(_ context.Context, name string) (string, error) {
	if v, ok := f.readBack[name]; ok {
		return v, nil
	}
	// Default to echoing what was written, which is what a healthy server does.
	return f.lastPrefValue, nil
}

func (f *fakeGame) Execute(_ context.Context, command string) (sdtd.CommandResult, error) {
	f.executed = append(f.executed, command)
	if fields := strings.Fields(command); len(fields) == 3 && fields[0] == "setgamepref" {
		f.lastPrefValue = fields[2]
	}
	if f.err != nil {
		return sdtd.CommandResult{}, f.err
	}
	if f.result.Command == "" {
		return sdtd.CommandResult{Command: command, Result: "ok"}, nil
	}
	return f.result, nil
}

type fakeCatalogue struct {
	items []sdtd.Command
	err   error
}

func (f *fakeCatalogue) Get(context.Context) ([]sdtd.Command, time.Time, error) {
	if f.err != nil {
		return nil, time.Time{}, f.err
	}
	return f.items, time.Unix(1_760_000_000, 0).UTC(), nil
}

func strptr(s string) *string { return &s }
func boolptr(b bool) *bool    { return &b }

func TestConsoleCommandsCarriesServerHelpAndPanelTier(t *testing.T) {
	h := newHarness(t, state.Snapshot{})
	h.srv.Commands = &fakeCatalogue{items: []sdtd.Command{
		{Command: "gettime", Overloads: []string{"gettime"}, Description: "shows time",
			Help: strptr("Usage:\n  gettime"), Allowed: boolptr(true)},
		{Command: "shutdown", Overloads: []string{"shutdown"}, Description: "stops the server"},
		{Command: "settime", Overloads: []string{"settime", "st"}, Description: "sets time"},
	}}
	cookie := h.login(t, "admin", testPassword)

	rec := h.do(t, h.request(t, http.MethodGet, "/api/console/commands", "", cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Commands []commandInfo `json:"commands"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Commands) != 3 {
		t.Fatalf("got %d commands, want 3", len(body.Commands))
	}

	byName := map[string]commandInfo{}
	for _, c := range body.Commands {
		byName[c.Name] = c
	}

	// The palette is built from the server's own catalogue, so its help text
	// has to survive the trip.
	if got := byName["gettime"].Help; !strings.Contains(got, "Usage") {
		t.Errorf("help = %q, want the server's usage text", got)
	}
	if got := byName["gettime"].Tier; got != "normal" {
		t.Errorf("gettime tier = %q, want normal", got)
	}
	if got := byName["shutdown"].Tier; got != "destructive" {
		t.Errorf("shutdown tier = %q, want destructive", got)
	}
	if got := byName["settime"].Tier; got != "mutating" {
		t.Errorf("settime tier = %q, want mutating", got)
	}
	// A command with no explicit allowed flag should not be shown as forbidden.
	if !byName["settime"].Allowed {
		t.Error("settime should default to allowed when the server omits the flag")
	}
	if len(byName["settime"].Aliases) != 2 {
		t.Errorf("aliases = %v, want both overloads", byName["settime"].Aliases)
	}
}

func TestConsoleCommandsMarksBlockedWhenDestructiveDisabled(t *testing.T) {
	h := newHarness(t, state.Snapshot{})
	h.server.cfg.Panel.AllowDestructive = false
	h.srv.Commands = &fakeCatalogue{items: []sdtd.Command{
		{Command: "shutdown", Description: "stops the server"},
		{Command: "gettime", Description: "shows time"},
	}}
	cookie := h.login(t, "admin", testPassword)

	rec := h.do(t, h.request(t, http.MethodGet, "/api/console/commands", "", cookie))
	var body struct {
		Commands []commandInfo `json:"commands"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)

	for _, c := range body.Commands {
		switch c.Name {
		case "shutdown":
			if !c.Blocked {
				t.Error("shutdown should be marked blocked")
			}
		case "gettime":
			if c.Blocked {
				t.Error("gettime should not be blocked")
			}
		}
	}
}

func TestConsoleExecuteRunsAndRecordsHistory(t *testing.T) {
	h := newHarness(t, state.Snapshot{})
	game := &fakeGame{result: sdtd.CommandResult{
		Command: "gettime", Parameters: "", Result: "Day 1, 07:00\n",
	}}
	h.srv.Client = game
	cookie := h.login(t, "admin", testPassword)

	rec := h.do(t, h.request(t, http.MethodPost, "/api/console/execute",
		`{"command":"gettime"}`, cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}

	var body executeResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Result != "Day 1, 07:00\n" {
		t.Errorf("result = %q", body.Result)
	}
	if body.Tier != "normal" {
		t.Errorf("tier = %q, want normal", body.Tier)
	}
	if len(game.executed) != 1 || game.executed[0] != "gettime" {
		t.Errorf("executed = %v, want [gettime]", game.executed)
	}

	// It should now appear in history.
	histRec := h.do(t, h.request(t, http.MethodGet, "/api/console/history", "", cookie))
	var hist struct {
		History []historyEntry `json:"history"`
	}
	_ = json.Unmarshal(histRec.Body.Bytes(), &hist)
	if len(hist.History) != 1 {
		t.Fatalf("history = %d entries, want 1", len(hist.History))
	}
	if hist.History[0].Command != "gettime" || !hist.History[0].Succeeded {
		t.Errorf("history entry = %+v", hist.History[0])
	}
}

func TestConsoleExecuteSurfacesTheRealServerError(t *testing.T) {
	h := newHarness(t, state.Snapshot{})
	h.srv.Client = &fakeGame{err: &sdtd.APIError{
		Status:           http.StatusNotFound,
		ErrorCode:        sdtd.CodeUnknownCommand,
		ExceptionMessage: "",
		Trace:            "at Webserver.Foo()",
	}}
	cookie := h.login(t, "admin", testPassword)

	rec := h.do(t, h.request(t, http.MethodPost, "/api/console/execute",
		`{"command":"definitelynotacommand"}`, cookie))

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
	body := decode[map[string]any](t, rec)
	// The brief is explicit: show the real error, never a generic failure.
	if body["code"] != sdtd.CodeUnknownCommand {
		t.Errorf("code = %v, want %s", body["code"], sdtd.CodeUnknownCommand)
	}
	// The stack trace belongs in the panel's log, not the browser.
	if strings.Contains(rec.Body.String(), "Webserver.Foo") {
		t.Error("the response leaked the stack trace")
	}

	// A failure must still be recorded, so the operator can see what they tried.
	histRec := h.do(t, h.request(t, http.MethodGet, "/api/console/history", "", cookie))
	var hist struct {
		History []historyEntry `json:"history"`
	}
	_ = json.Unmarshal(histRec.Body.Bytes(), &hist)
	if len(hist.History) != 1 || hist.History[0].Succeeded {
		t.Fatalf("history = %+v, want one failed entry", hist.History)
	}
	if hist.History[0].Error == "" {
		t.Error("the failure reason was not recorded")
	}
}

func TestConsoleExecuteRejectsBadInput(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		wantCode int
		wantErr  string
	}{
		{"empty command", `{"command":""}`, http.StatusBadRequest, "INVALID_COMMAND"},
		{"whitespace only", `{"command":"   "}`, http.StatusBadRequest, "INVALID_COMMAND"},
		{"embedded newline", `{"command":"gettime\nshutdown"}`, http.StatusBadRequest, "INVALID_COMMAND"},
		{"malformed json", `{"command":`, http.StatusBadRequest, "INVALID_BODY"},
		{"unknown field", `{"command":"gettime","sudo":true}`, http.StatusBadRequest, "INVALID_BODY"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t, state.Snapshot{})
			game := &fakeGame{}
			h.srv.Client = game
			cookie := h.login(t, "admin", testPassword)

			rec := h.do(t, h.request(t, http.MethodPost, "/api/console/execute", tt.body, cookie))
			if rec.Code != tt.wantCode {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantCode)
			}
			if got := decode[map[string]any](t, rec)["code"]; got != tt.wantErr {
				t.Errorf("code = %v, want %v", got, tt.wantErr)
			}
			if len(game.executed) != 0 {
				t.Errorf("a rejected command still reached the game server: %v", game.executed)
			}
		})
	}
}

func TestDestructiveCommandBlockedByConfig(t *testing.T) {
	h := newHarness(t, state.Snapshot{})
	h.server.cfg.Panel.AllowDestructive = false
	game := &fakeGame{}
	h.srv.Client = game
	cookie := h.login(t, "admin", testPassword)

	rec := h.do(t, h.request(t, http.MethodPost, "/api/console/execute",
		`{"command":"shutdown"}`, cookie))

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
	if got := decode[map[string]any](t, rec)["code"]; got != "DESTRUCTIVE_BLOCKED" {
		t.Errorf("code = %v, want DESTRUCTIVE_BLOCKED", got)
	}
	if len(game.executed) != 0 {
		t.Error("a blocked command still reached the game server")
	}
}

func TestDestructiveCommandRunsWhenAllowed(t *testing.T) {
	h := newHarness(t, state.Snapshot{})
	h.server.cfg.Panel.AllowDestructive = true
	game := &fakeGame{}
	h.srv.Client = game
	cookie := h.login(t, "admin", testPassword)

	rec := h.do(t, h.request(t, http.MethodPost, "/api/console/execute",
		`{"command":"shutdown"}`, cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if got := decode[map[string]any](t, rec)["tier"]; got != "destructive" {
		t.Errorf("tier = %v, want destructive so the UI can demand confirmation", got)
	}
}

func TestConsoleEndpointsRequireAuth(t *testing.T) {
	h := newHarness(t, state.Snapshot{})
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/console/commands", ""},
		{http.MethodGet, "/api/console/history", ""},
		{http.MethodPost, "/api/console/execute", `{"command":"gettime"}`},
		{http.MethodGet, "/api/events", ""},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			rec := h.do(t, h.request(t, tc.method, tc.path, tc.body, nil))
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401", rec.Code)
			}
		})
	}
}

func TestCommandCatalogueFailureIsRelayed(t *testing.T) {
	h := newHarness(t, state.Snapshot{})
	h.srv.Commands = &fakeCatalogue{err: errors.New("connection refused")}
	cookie := h.login(t, "admin", testPassword)

	rec := h.do(t, h.request(t, http.MethodGet, "/api/console/commands", "", cookie))
	if rec.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "connection refused") {
		t.Errorf("body should carry the real reason: %s", rec.Body.String())
	}
}

func TestEventsStreamsBacklogThenLiveEvents(t *testing.T) {
	hub := events.NewHub()
	hub.PublishStatus("earlier event")

	h := newHarness(t, state.Snapshot{})
	h.srv.Events = hub
	cookie := h.login(t, "admin", testPassword)

	req := h.request(t, http.MethodGet, "/api/events?backlog=10", "", cookie)
	ctx, cancel := context.WithCancel(req.Context())
	req = req.WithContext(ctx)

	rec := newStreamRecorder()
	done := make(chan struct{})
	go func() {
		h.handler.ServeHTTP(rec, req)
		close(done)
	}()

	// Backlog first.
	rec.waitFor(t, "earlier event", 3*time.Second)

	// Then anything published afterwards.
	hub.PublishStatus("later event")
	rec.waitFor(t, "later event", 3*time.Second)

	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}
	if got := rec.Header().Get("Cache-Control"); !strings.Contains(got, "no-cache") {
		t.Errorf("Cache-Control = %q, want no-cache", got)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("the handler did not return when the client disconnected")
	}
}

func TestEventsRejectsBadBacklog(t *testing.T) {
	h := newHarness(t, state.Snapshot{})
	h.srv.Events = events.NewHub()
	cookie := h.login(t, "admin", testPassword)

	rec := h.do(t, h.request(t, http.MethodGet, "/api/events?backlog=-5", "", cookie))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

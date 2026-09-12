package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/nomansheikh/7dtd-panel/internal/sdtd"
)

// Everything under /access puts an identifier straight into a console command,
// so a malformed one has to be refused before it reaches the game rather than
// after.
func TestAccessEndpointsRefuseMalformedIDs(t *testing.T) {
	for _, tc := range []struct{ name, path, body string }{
		{"set admin", "/api/access/admin", `{"platformUserId":"Steam_1 say hi","level":0}`},
		{"remove admin", "/api/access/admin/remove", `{"platformUserId":"nope"}`},
		{"add whitelist", "/api/access/whitelist", `{"platformUserId":""}`},
		{"remove land claims", "/api/land-claims/remove", `{"platformUserId":"; shutdown"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, game, cookie := worldHarness(t)

			rec := h.do(t, h.request(t, http.MethodPost, tc.path, tc.body, cookie))
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", rec.Code)
			}
			if len(game.executed) != 0 {
				t.Errorf("the game was sent %q", game.executed)
			}
		})
	}
}

func TestSetMaxPlayersRefusesAnAbsurdCap(t *testing.T) {
	h, game, cookie := worldHarness(t)

	rec := h.do(t, h.request(t, http.MethodPost, "/api/max-players", `{"count":0}`, cookie))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if len(game.executed) != 0 {
		t.Errorf("the game was sent %q", game.executed)
	}
}

// The inventory reply is handed back as the game wrote it, because the format
// could not be checked against a live server and a parser for output nobody
// has seen would produce confident wrong answers.
func TestInventoryIsReturnedAsWritten(t *testing.T) {
	h, game, cookie := worldHarness(t)
	game.result = sdtd.CommandResult{
		Command: "showinventory 171",
		Result:  "Playername or entity/steamid id not found or no inventory saved",
	}

	rec := h.do(t, h.request(t, http.MethodGet, "/api/players/171/inventory", "", cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Command string `json:"command"`
		Text    string `json:"text"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Command != "showinventory 171" {
		t.Errorf("command = %q", body.Command)
	}
	if !strings.Contains(body.Text, "no inventory saved") {
		t.Errorf("text = %q, want the game's own words", body.Text)
	}
}

// The icon name goes straight into another URL path, so it gets the same
// check the give command's item name does.
func TestItemIconRejectsATraversingName(t *testing.T) {
	h, game, cookie := worldHarness(t)

	rec := h.do(t, h.request(t, http.MethodGet, "/api/items/..%2F..%2Fsecret/icon", "", cookie))
	if rec.Code == http.StatusOK {
		t.Fatalf("status = %d, want a refusal", rec.Code)
	}
	if game.iconFor != "" {
		t.Errorf("the game was asked for %q", game.iconFor)
	}
}

func TestItemIconIsServedAndCached(t *testing.T) {
	h, game, cookie := worldHarness(t)
	game.icon = []byte("\x89PNG\r\n\x1a\n fake")

	rec := h.do(t, h.request(t, http.MethodGet, "/api/items/meleeToolAxeT2SteelAxe/icon", "", cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "image/png" {
		t.Errorf("content type = %q", got)
	}
	// The art belongs to the game build, so a picker showing a hundred of them
	// must not re-fetch on every keystroke.
	if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "max-age=") {
		t.Errorf("cache-control = %q, want it cached", cc)
	}
	if game.iconFor != "meleeToolAxeT2SteelAxe" {
		t.Errorf("asked the game for %q", game.iconFor)
	}
}

// A catalogue entry with no art is ordinary, not an error worth a 502.
func TestItemIconMissingIsNotFound(t *testing.T) {
	h, game, cookie := worldHarness(t)
	game.iconErr = sdtd.ErrNoIcon

	rec := h.do(t, h.request(t, http.MethodGet, "/api/items/recipeThing/icon", "", cookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

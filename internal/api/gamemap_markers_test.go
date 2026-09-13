package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/nomansheikh/7dtd-panel/internal/sdtd"
)

// The overlays: which layers get fetched, what one layer failing does to the
// others, and the shapes the map has to be able to iterate. The harness and
// the tile tests live in gamemap_test.go.

type markerLayersBody struct {
	Layers   map[string][]marker `json:"layers"`
	Problems map[string]string   `json:"problems"`
}

func TestTheMapAsksForOnlyTheLayersItIsShowing(t *testing.T) {
	h, game, cookie := mapHarness(t)
	game.players = []sdtd.Player{{
		EntityID: 1, Name: "Ana", Online: true, PlatformID: "Steam_1",
		Position: sdtd.Position{X: 10, Y: 61, Z: -20},
	}}
	game.hostiles = []sdtd.Entity{{ID: 500, Name: "zombieBiker"}}

	rec := h.do(t, h.request(t, http.MethodGet, "/api/map/markers?layers=players", "", cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	got := decode[markerLayersBody](t, rec)

	if len(got.Layers["players"]) != 1 {
		t.Fatalf("players layer has %d markers, want 1", len(got.Layers["players"]))
	}
	if _, ok := got.Layers["hostiles"]; ok {
		t.Fatal("returned a layer nobody asked for; on a blood moon that is " +
			"hundreds of entries fetched for nothing")
	}
	if player := got.Layers["players"][0]; player.Name != "Ana" || player.X != 10 || player.Z != -20 {
		t.Fatalf("got %+v", player)
	}
}

// Degrade, don't crash: one failing overlay must not blank out the rest.
func TestOneFailingLayerStillLetsTheOthersDraw(t *testing.T) {
	h, game, cookie := mapHarness(t)
	game.players = []sdtd.Player{{EntityID: 1, Name: "Ana", Online: true}}
	game.hostilesErr = &sdtd.APIError{Status: http.StatusInternalServerError, ErrorCode: "BOOM"}

	rec := h.do(t, h.request(t, http.MethodGet, "/api/map/markers?layers=players,hostiles", "", cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	got := decode[markerLayersBody](t, rec)

	if len(got.Layers["players"]) != 1 {
		t.Fatalf("the player layer was lost to another layer's failure: %+v", got.Layers)
	}
	if got.Problems["hostiles"] == "" {
		t.Fatal("want the zombie layer's failure named, not swallowed")
	}
}

func TestOfflinePlayersAreNotPlacedOnTheMap(t *testing.T) {
	h, game, cookie := mapHarness(t)
	game.players = []sdtd.Player{
		{EntityID: 1, Name: "Ana", Online: true},
		{EntityID: 2, Name: "Bo", Online: false},
	}

	rec := h.do(t, h.request(t, http.MethodGet, "/api/map/markers?layers=players", "", cookie))
	got := decode[markerLayersBody](t, rec)

	if len(got.Layers["players"]) != 1 || got.Layers["players"][0].Name != "Ana" {
		t.Fatalf("got %+v, want only the online player", got.Layers["players"])
	}
}

func TestALandClaimCarriesTheSquareItProtects(t *testing.T) {
	h, game, cookie := mapHarness(t)
	game.claims = []sdtd.LandClaim{{
		PlatformID: "Steam_1", Owner: "Ana", Active: true, Size: 41,
		Position: sdtd.Position{X: 100, Y: 61, Z: 200},
	}}

	rec := h.do(t, h.request(t, http.MethodGet, "/api/map/markers?layers=claims", "", cookie))
	got := decode[markerLayersBody](t, rec)

	claims := got.Layers["claims"]
	if len(claims) != 1 {
		t.Fatalf("claims layer has %d markers, want 1", len(claims))
	}
	if claims[0].Size != 41 || !claims[0].Active || claims[0].Owner != "Ana" {
		t.Fatalf("got %+v", claims[0])
	}
}

// A nil slice marshals to null, which the map would try to iterate.
func TestAnEmptyLayerIsAnEmptyListNotNull(t *testing.T) {
	h, _, cookie := mapHarness(t)

	rec := h.do(t, h.request(t, http.MethodGet, "/api/map/markers?layers=players", "", cookie))
	body := decode[map[string]json.RawMessage](t, rec)

	var layers map[string]json.RawMessage
	if err := json.Unmarshal(body["layers"], &layers); err != nil {
		t.Fatalf("layers: %v", err)
	}
	if string(layers["players"]) != "[]" {
		t.Fatalf("players layer is %s, want []", layers["players"])
	}
}

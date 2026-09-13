package api

import (
	"net/http"
	"strings"
	"testing"

	"github.com/nomansheikh/7dtd-panel/internal/gamemap"
	"github.com/nomansheikh/7dtd-panel/internal/sdtd"
	"github.com/nomansheikh/7dtd-panel/internal/state"
)

// mapHarness gives each test its own game and its own tile cache, so one
// test's cached tiles cannot answer another test's request.
func mapHarness(t *testing.T) (*harness, *fakeGame, *http.Cookie) {
	t.Helper()
	h := newHarness(t, state.Snapshot{Status: state.StatusOnline})
	game := &fakeGame{}
	h.srv.Client = game
	h.srv.Map = gamemap.New(gamemap.Options{Source: game})
	return h, game, h.login(t, "admin", testPassword)
}

func TestTheMapConfigIsReportedInThePanelsOwnTerms(t *testing.T) {
	h, game, cookie := mapHarness(t)
	game.mapConfig = sdtd.MapConfig{
		Enabled:   true,
		BlockSize: 128,
		MaxZoom:   4,
		MapSize:   sdtd.Position{X: 6144, Y: 255, Z: 6144},
	}

	rec := h.do(t, h.request(t, http.MethodGet, "/api/map/config", "", cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}

	got := decode[struct {
		Enabled   bool `json:"enabled"`
		TileSize  int  `json:"tileSize"`
		MaxZoom   int  `json:"maxZoom"`
		WorldSize int  `json:"worldSize"`
	}](t, rec)

	if !got.Enabled || got.TileSize != 128 || got.MaxZoom != 4 || got.WorldSize != 6144 {
		t.Fatalf("got %+v", got)
	}
}

// The browser counts tile rows the opposite way to the game. Getting this
// wrong mirrors the whole world north to south, which nobody notices until
// they go looking for a base.
func TestATilesRowIsFlippedIntoTheGamesNumbering(t *testing.T) {
	h, game, cookie := mapHarness(t)
	game.tile = []byte("\x89PNG fake")

	h.do(t, h.request(t, http.MethodGet, "/api/map/tiles/4/7/3", "", cookie))

	want := gamemap.Key{Z: 4, X: 7, Y: -4}
	if game.tileFor != want {
		t.Fatalf("asked the game server for %+v, want %+v", game.tileFor, want)
	}
}

func TestATileIsServedAsAPNGTheBrowserMayCache(t *testing.T) {
	h, game, cookie := mapHarness(t)
	game.tile = []byte("\x89PNG fake")

	rec := h.do(t, h.request(t, http.MethodGet, "/api/map/tiles/0/0/0", "", cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}

	if ct := rec.Header().Get("Content-Type"); ct != "image/png" {
		t.Fatalf("Content-Type %q, want image/png", ct)
	}
	if rec.Header().Get("ETag") == "" {
		t.Fatal("want an ETag so the browser can revalidate rather than refetch")
	}
	cc := rec.Header().Get("Cache-Control")
	if !strings.Contains(cc, "max-age=") {
		t.Fatalf("Cache-Control %q, want a max-age", cc)
	}
	// The map gives away where every player and every base is. It must not
	// land in a cache anybody else can read.
	if !strings.Contains(cc, "private") {
		t.Fatalf("Cache-Control %q, want it marked private", cc)
	}
}

func TestARevalidatedTileAnswersNotModified(t *testing.T) {
	h, game, cookie := mapHarness(t)
	game.tile = []byte("\x89PNG fake")

	first := h.do(t, h.request(t, http.MethodGet, "/api/map/tiles/2/1/1", "", cookie))
	etag := first.Header().Get("ETag")

	req := h.request(t, http.MethodGet, "/api/map/tiles/2/1/1", "", cookie)
	req.Header.Set("If-None-Match", etag)
	second := h.do(t, req)

	if second.Code != http.StatusNotModified {
		t.Fatalf("status %d, want 304", second.Code)
	}
	if second.Body.Len() != 0 {
		t.Fatalf("a 304 carried %d bytes of body", second.Body.Len())
	}
}

// Most of a world has never been drawn, because the renderer only draws a
// square once a player has loaded the chunks under it.
func TestAnUnrenderedSquareIsAnEmptyAnswerNotAnError(t *testing.T) {
	h, game, cookie := mapHarness(t)
	game.tileErr = sdtd.ErrNoTile

	rec := h.do(t, h.request(t, http.MethodGet, "/api/map/tiles/4/99/99", "", cookie))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status %d, want 204: %s", rec.Code, rec.Body.String())
	}
	if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "max-age=") {
		t.Fatalf("Cache-Control %q: a blank square has to be cacheable too, or "+
			"panning an unexplored map is thousands of requests", cc)
	}
}

func TestANonsenseTileCoordinateIsRefused(t *testing.T) {
	h, _, cookie := mapHarness(t)

	for _, path := range []string{
		"/api/map/tiles/x/0/0",
		"/api/map/tiles/0/y/0",
		"/api/map/tiles/-1/0/0",
		"/api/map/tiles/0/99999999/0",
	} {
		rec := h.do(t, h.request(t, http.MethodGet, path, "", cookie))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", path, rec.Code)
		}
	}
}

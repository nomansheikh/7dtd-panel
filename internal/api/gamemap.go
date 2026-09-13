package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/nomansheikh/7dtd-panel/internal/gamemap"
	"github.com/nomansheikh/7dtd-panel/internal/httpx"
)

// handleMapConfig reports the map's dimensions.
//
// The browser cannot place a single tile without these: the zoom range, the
// tile size and the world's extent are all decided by the game's renderer, not
// by the panel, and they differ between a Navezgane map and an 8k random one.
func (s *Server) handleMapConfig(w http.ResponseWriter, r *http.Request) {
	config, err := serverFrom(r.Context()).Client.MapConfig(r.Context())
	if err != nil {
		s.writeGameError(w, err, "could not read the map configuration")
		return
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"enabled":   config.Enabled,
		"tileSize":  config.BlockSize,
		"maxZoom":   config.MaxZoom,
		"worldSize": config.MapSize.X,
	})
}

/*
Far above the 128 tiles a side that the largest world needs, and still
small enough that nothing below can overflow.
*/
const maxTileCoordinate = 1 << 16

// handleMapTile serves one rendered square.
//
// Proxied rather than linked, like item art and for the same reason: on most
// installs the game server sits on a private network the operator's laptop
// cannot reach, and the panel is the only thing holding the token.
//
// Everything about the caching is the panel's own. The game server sends no
// validator and no cache directive on tiles at all — its own map client works
// around that by appending a fresh timestamp to every URL, which means a full
// refetch of the viewport on every page load. Here the URL stays stable, the
// ETag comes from the bytes, and max-age comes from whether the world is
// occupied, so a panel left open on an empty server asks for nothing.
func (s *Server) handleMapTile(w http.ResponseWriter, r *http.Request) {
	key, ok := tileKey(w, r)
	if !ok {
		return
	}

	tile, err := serverFrom(r.Context()).Map.Get(r.Context(), key)
	if err != nil {
		s.writeGameError(w, err, "could not load that part of the map")
		return
	}

	w.Header().Set("ETag", tile.ETag)
	w.Header().Set("Cache-Control", cacheControl(tile.MaxAge))
	// A tile is only ever this operator's to see: the map can give away where
	// every player and base is, so it must not land in a shared cache.
	w.Header().Set("Vary", "Cookie")

	if match := r.Header.Get("If-None-Match"); match != "" && match == tile.ETag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	if tile.Missing {
		/* Ordinary, not exceptional: most of a world has never been drawn. */
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Length", strconv.Itoa(len(tile.Bytes)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(tile.Bytes)
}

/*
stale-while-revalidate lets the browser paint the square it has and check
for a newer one behind, so panning over covered ground never waits.
*/
func cacheControl(age time.Duration) string {
	seconds := int(age.Seconds())
	if seconds < 1 {
		seconds = 1
	}
	return "private, max-age=" + strconv.Itoa(seconds) + ", stale-while-revalidate=60"
}

/*
The y in the path is the browser's row, which counts the opposite way to
the game's. Flipped here so the URL in the network tab matches the tile file
on the game server.
*/
func tileKey(w http.ResponseWriter, r *http.Request) (gamemap.Key, bool) {
	z, zOK := tileCoordinate(r.PathValue("z"))
	x, xOK := tileCoordinate(r.PathValue("x"))
	y, yOK := tileCoordinate(r.PathValue("y"))
	if !zOK || !xOK || !yOK {
		httpx.WriteError(w, http.StatusBadRequest, "that is not a map tile", "INVALID_TILE")
		return gamemap.Key{}, false
	}
	if z < 0 {
		httpx.WriteError(w, http.StatusBadRequest, "that is not a map tile", "INVALID_TILE")
		return gamemap.Key{}, false
	}
	return gamemap.Key{Z: z, X: x, Y: -y - 1}, true
}

func tileCoordinate(raw string) (int, bool) {
	n, err := strconv.Atoi(raw)
	if err != nil || n < -maxTileCoordinate || n > maxTileCoordinate {
		return 0, false
	}
	return n, true
}

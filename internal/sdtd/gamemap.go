package sdtd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
)

// ErrNoTile means the renderer has never drawn that square. It is the ordinary
// answer for most of a world: the game only writes a tile once a player has
// loaded the chunks under it, so a fresh map is almost entirely missing.
var ErrNoTile = errors.New("sdtd: no tile has been rendered there")

// MapConfig describes the rendered map, from /api/map/config.
//
// MaxZoom is the deepest level the renderer produces, at which one world block
// is one pixel. MapSize is in blocks, so the tile grid at MaxZoom is
// MapSize.X/BlockSize squares on a side.
type MapConfig struct {
	Enabled   bool     `json:"enabled"`
	BlockSize int      `json:"mapBlockSize"`
	MaxZoom   int      `json:"maxZoom"`
	MapSize   Position `json:"mapSize"`
}

// MapConfig fetches the map's dimensions.
//
// Enabled reports the EnableMapRendering preference. It can be false while
// tiles still exist from an earlier session, and true while no tiles exist at
// all, so it says whether the map will keep growing, not whether there is
// anything to look at.
func (c *Client) MapConfig(ctx context.Context) (MapConfig, error) {
	return fetch[MapConfig](func() (*http.Response, error) {
		return c.gen.MapGetConfig(ctx)
	})
}

// maxTileBytes bounds one tile. A 128x128 PNG of terrain is 5-15 kB; the
// ceiling is set well above that so an unusual tile still loads, and far below
// anything that would let a wrong path pull a large file into memory.
const maxTileBytes = 2 << 20

// MapTile fetches one rendered square as a PNG.
//
// The tiles are not part of the documented API and so are not in the generated
// client: they are served as plain files from /map/{z}/{x}/{y}.png, outside
// /api, with no envelope. The path was read from the server's own map client
// and confirmed against the copy the game ships in
// Mods/Allocs_WebAndMapRendering.
//
// y is the tile row in the game's own numbering. The caller flips it; see
// TileY on the panel side.
func (c *Client) MapTile(ctx context.Context, z, x, y int) ([]byte, error) {
	url := c.baseURL + "/map/" + strconv.Itoa(z) + "/" + strconv.Itoa(x) + "/" + strconv.Itoa(y) + ".png"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("sdtd: map tile: %w", err)
	}
	// The tile files are served by the same web server as the API and honour
	// the same credentials. Sent unconditionally because a server configured to
	// keep its map private will otherwise refuse them.
	req.Header.Set(HeaderTokenName, c.tokenName)
	req.Header.Set(HeaderTokenSecret, c.tokenSecret)
	req.Header.Set("Accept", "image/png")

	resp, err := c.tiles.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sdtd: map tile: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNoTile
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, &APIError{Status: resp.StatusCode, ErrorCode: "NOT_AUTHORIZED"}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sdtd: map tile: unexpected status %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxTileBytes))
}

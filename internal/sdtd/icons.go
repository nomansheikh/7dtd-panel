package sdtd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// DefaultIconTint is the tint the game applies when nothing else is asked for.
//
// The path is /itemicons/{name}__{tint}.png and the tint is not optional: the
// same name without one is a 404, which is what made icons look unavailable.
const DefaultIconTint = "FFFFFF"

// ErrNoIcon means the game has no art for that name, which is ordinary: many
// entries in the catalogue are recipes or internal items nobody ever holds.
var ErrNoIcon = errors.New("sdtd: no icon for that item")

// ItemIcon fetches one item's icon as a PNG.
//
// Returned as bytes rather than a stream because the caller caches them: the
// art is part of the game build and does not change while the server is up.
func (c *Client) ItemIcon(ctx context.Context, name, tint string) ([]byte, error) {
	if tint == "" {
		tint = DefaultIconTint
	}
	res, err := c.gen.ItemIconHandlerGetName(ctx, name, tint)
	if err != nil {
		return nil, fmt.Errorf("sdtd: item icon: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusNotFound {
		return nil, ErrNoIcon
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sdtd: item icon: unexpected status %d", res.StatusCode)
	}
	// Bounded so a wrong path cannot pull the whole item catalogue into memory
	// as if it were a picture: the largest icon observed is about 45 kB.
	return io.ReadAll(io.LimitReader(res.Body, 512<<10))
}

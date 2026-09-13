package sdtd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Entity is one non-player thing standing in the world.
//
// Hostiles and animals come back in the same shape from two endpoints, so they
// share a type; what distinguishes them is which call produced them.
type Entity struct {
	ID       int      `json:"id"`
	Name     string   `json:"name"`
	Position Position `json:"position"`
}

// Hostiles lists the zombies the server currently has loaded.
//
// Only entities in loaded chunks exist, so this is empty on an idle server and
// can run to hundreds during a blood moon.
func (c *Client) Hostiles(ctx context.Context) ([]Entity, error) {
	return fetch[[]Entity](func() (*http.Response, error) {
		return c.gen.HostileGet(ctx)
	})
}

// Animals lists the loaded wildlife, on the same terms as Hostiles.
func (c *Client) Animals(ctx context.Context) ([]Entity, error) {
	return fetch[[]Entity](func() (*http.Response, error) {
		return c.gen.AnimalGet(ctx)
	})
}

// LandClaim is one claim block, with the owner it belongs to.
//
// Size is the claim's full width in blocks, carried on every claim because it
// is a server preference rather than a property of the block, and drawing the
// protected square needs it.
type LandClaim struct {
	PlatformID string   `json:"platformId"`
	Owner      string   `json:"owner"`
	Active     bool     `json:"active"`
	Position   Position `json:"position"`
	Size       int      `json:"size"`
}

// landClaimsResponse mirrors /api/getlandclaims.
//
// That endpoint is absent from the OpenAPI spec. Its field names were read from
// the map client the game ships in Mods/Allocs_WebAndMapRendering and confirmed
// against a live server. Unlike the documented endpoints it has no {data, meta}
// envelope, which is why this does not go through fetch.
type landClaimsResponse struct {
	ClaimSize int `json:"claimsize"`
	Owners    []struct {
		SteamID    string `json:"steamid"`
		PlayerName string `json:"playername"`
		Active     bool   `json:"claimactive"`
		Claims     []struct {
			X float64 `json:"x"`
			Y float64 `json:"y"`
			Z float64 `json:"z"`
		} `json:"claims"`
	} `json:"claimowners"`
}

// LandClaims lists every claim block on the server, flattened from the
// owner-grouped shape the game returns.
func (c *Client) LandClaims(ctx context.Context) ([]LandClaim, error) {
	var body landClaimsResponse
	if err := c.getLegacyJSON(ctx, "/api/getlandclaims", &body); err != nil {
		return nil, err
	}

	claims := make([]LandClaim, 0, len(body.Owners))
	for _, owner := range body.Owners {
		for _, claim := range owner.Claims {
			claims = append(claims, LandClaim{
				PlatformID: owner.SteamID,
				Owner:      owner.PlayerName,
				Active:     owner.Active,
				Position:   Position{X: claim.X, Y: claim.Y, Z: claim.Z},
				Size:       body.ClaimSize,
			})
		}
	}
	return claims, nil
}

// getLegacyJSON reads one of the undocumented endpoints, which answer with a
// bare JSON value rather than the {data, meta} envelope every documented one
// uses.
func (c *Client) getLegacyJSON(ctx context.Context, path string, into any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("sdtd: %s: %w", path, err)
	}
	req.Header.Set(HeaderTokenName, c.tokenName)
	req.Header.Set(HeaderTokenSecret, c.tokenSecret)
	req.Header.Set("Accept", "application/json")

	resp, err := c.tiles.Do(req)
	if err != nil {
		return fmt.Errorf("sdtd: %s: %w", path, err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return &APIError{Status: resp.StatusCode, RequestSubpath: path}
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return fmt.Errorf("sdtd: %s: read body: %w", path, err)
	}
	if err := json.Unmarshal(body, into); err != nil {
		return fmt.Errorf("sdtd: %s: decode response: %w", path, err)
	}
	return nil
}

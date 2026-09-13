package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"

	"github.com/nomansheikh/7dtd-panel/internal/httpx"
	"github.com/nomansheikh/7dtd-panel/internal/sdtd"
	"github.com/nomansheikh/7dtd-panel/internal/servers"
)

// marker is one thing drawn on the map. The axes are the game's: x runs east,
// z runs north, and y is height, which the map ignores but a popup shows.
type marker struct {
	ID    int     `json:"id"`
	Name  string  `json:"name"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Z     float64 `json:"z"`
	Owner string  `json:"owner,omitempty"`
	// PlatformID lets the map link a player marker to its player page.
	PlatformID string `json:"platformId,omitempty"`
	// Size is a land claim's protected width in blocks. Zero for everything
	// else, which is drawn as a point.
	Size int `json:"size,omitempty"`
	// Active distinguishes a live land claim from a lapsed one.
	Active bool `json:"active,omitempty"`
}

/*
handleMapMarkers returns everything overlaid on the map in one answer.

One request rather than four, because the map redraws all its overlays
together: four separate polls would trip over each other and show players from
one moment beside zombies from another. The layers to include are named by the
caller so a map with the zombie layer switched off does not make the panel ask
the game server for zombies every few seconds — on a blood moon that list runs
to hundreds of entries that nobody has asked to see.

A layer that fails is reported as a failure for that layer alone. A game server
that has stopped answering /api/hostile should not blank out the player
positions the panel can still read perfectly well.
*/
func (s *Server) handleMapMarkers(w http.ResponseWriter, r *http.Request) {
	wanted := requestedLayers(r.URL.Query().Get("layers"))
	server := serverFrom(r.Context())

	layers := map[string][]marker{}
	problems := map[string]string{}
	var mu sync.Mutex
	var wg sync.WaitGroup

	for name, collect := range markerLayers {
		if !wanted[name] {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			found, err := collect(r.Context(), server)

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				problems[name] = friendlyGameError(err)
				return
			}
			layers[name] = found
		}()
	}
	wg.Wait()

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"layers":   layers,
		"problems": problems,
	})
}

// friendlyGameError renders one layer's failure for the operator.
//
// Per layer rather than per request, so the map can say "the zombie list is
// unavailable" while still drawing everything it did manage to read.
func friendlyGameError(err error) string {
	var apiErr *sdtd.APIError
	if errors.As(err, &apiErr) {
		return apiErr.SafeMessage()
	}
	return "the game server did not answer"
}

// markerLayers is every overlay the map can draw, by the name the browser asks
// for it under.
var markerLayers = map[string]func(context.Context, *servers.Server) ([]marker, error){
	"players":  collectPlayers,
	"hostiles": collectHostiles,
	"animals":  collectAnimals,
	"claims":   collectClaims,
}

// requestedLayers reads the layers parameter. An empty parameter means the two
// cheap layers, which is what the map opens with.
func requestedLayers(raw string) map[string]bool {
	if strings.TrimSpace(raw) == "" {
		return map[string]bool{"players": true, "claims": true}
	}
	wanted := map[string]bool{}
	for _, name := range strings.Split(raw, ",") {
		name = strings.TrimSpace(name)
		if _, ok := markerLayers[name]; ok {
			wanted[name] = true
		}
	}
	return wanted
}

// collectPlayers reads the online players. It reuses the panel's own merged
// player list rather than adding a call, so the map and the players page
// cannot disagree about who is where.
func collectPlayers(ctx context.Context, server *servers.Server) ([]marker, error) {
	players, err := server.Client.Players(ctx)
	if err != nil {
		return nil, err
	}
	found := make([]marker, 0, len(players))
	for _, player := range players {
		if !player.Online {
			continue
		}
		found = append(found, marker{
			ID:         player.EntityID,
			Name:       player.Name,
			X:          player.Position.X,
			Y:          player.Position.Y,
			Z:          player.Position.Z,
			PlatformID: player.PlatformID,
		})
	}
	return found, nil
}

func collectHostiles(ctx context.Context, server *servers.Server) ([]marker, error) {
	return collectEntities(server.Client.Hostiles(ctx))
}

func collectAnimals(ctx context.Context, server *servers.Server) ([]marker, error) {
	return collectEntities(server.Client.Animals(ctx))
}

func collectEntities(entities []sdtd.Entity, err error) ([]marker, error) {
	if err != nil {
		return nil, err
	}
	found := make([]marker, 0, len(entities))
	for _, entity := range entities {
		found = append(found, marker{
			ID:   entity.ID,
			Name: entity.Name,
			X:    entity.Position.X,
			Y:    entity.Position.Y,
			Z:    entity.Position.Z,
		})
	}
	return found, nil
}

func collectClaims(ctx context.Context, server *servers.Server) ([]marker, error) {
	claims, err := server.Client.LandClaims(ctx)
	if err != nil {
		return nil, err
	}
	found := make([]marker, 0, len(claims))
	for _, claim := range claims {
		found = append(found, marker{
			Name:       claim.Owner,
			X:          claim.Position.X,
			Y:          claim.Position.Y,
			Z:          claim.Position.Z,
			Owner:      claim.Owner,
			PlatformID: claim.PlatformID,
			Size:       claim.Size,
			Active:     claim.Active,
		})
	}
	return found, nil
}

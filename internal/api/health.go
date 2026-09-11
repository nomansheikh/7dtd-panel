package api

import (
	"net/http"

	"github.com/nomansheikh/7dtd-panel/internal/httpx"
	"github.com/nomansheikh/7dtd-panel/internal/state"
)

type healthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
	// Games reports each configured server's state as data, never as this
	// endpoint's status code.
	Games []healthGame `json:"games"`
}

type healthGame struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// handleHealth reports the panel's own liveness.
//
// It returns 200 even when every game server is unreachable, and deliberately
// so. The game servers are different machines; if their being down made this
// endpoint fail, Docker would restart the panel because something else broke.
// Their state is data in the body for the UI to read.
func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	all := s.registry.All()
	games := make([]healthGame, 0, len(all))

	for _, srv := range all {
		snap := srv.Poller.Snapshot()
		game := healthGame{ID: srv.ID, Name: srv.Name, Status: string(snap.Status)}
		// Surface the reason only once a server is actually considered down; a
		// single blip is not worth alarming about.
		if snap.Status == state.StatusOffline {
			game.Error = snap.LastError
		}
		games = append(games, game)
	}

	httpx.WriteJSON(w, http.StatusOK, healthResponse{
		Status:  "ok",
		Version: s.version,
		Games:   games,
	})
}

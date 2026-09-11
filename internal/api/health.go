package api

import (
	"net/http"

	"github.com/nomansheikh/7dtd-panel/internal/httpx"
	"github.com/nomansheikh/7dtd-panel/internal/state"
)

type healthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
	// Game reports the game server's state as data, never as this endpoint's
	// status code.
	Game healthGame `json:"game"`
}

type healthGame struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// handleHealth reports the panel's own liveness.
//
// It returns 200 even when the game server is unreachable, and deliberately so.
// The game server is a different machine; if its being down made this endpoint
// fail, Docker would restart the panel because something else broke. The game
// server's state is a field in the body for the UI to read.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	snap := s.state.Snapshot()

	resp := healthResponse{
		Status:  "ok",
		Version: s.version,
		Game: healthGame{
			Status: string(snap.Status),
		},
	}
	// Surface the real reason, but only once the server is actually considered
	// down; a single blip is not worth alarming about.
	if snap.Status == state.StatusOffline {
		resp.Game.Error = snap.LastError
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

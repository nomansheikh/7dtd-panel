package api

import (
	"net/http"

	"github.com/nomansheikh/7dtd-panel/internal/console"
	"github.com/nomansheikh/7dtd-panel/internal/httpx"
)

// Access control and the player operations that read or write raw console
// output rather than structured data.
//
// Several of these hand back the game's own text unparsed. That is deliberate
// where the format could not be checked against a live server: inventing a
// parser for output nobody has seen produces confident wrong answers, and the
// raw lines are useful on their own.

// platformRequest identifies an offline player, which is the only form the
// admin and whitelist commands accept for somebody not currently connected.
type platformRequest struct {
	PlatformUserID string `json:"platformUserId"`
}

func decodeBody[T any](w http.ResponseWriter, r *http.Request) (T, bool) {
	var req T
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error(), "INVALID_BODY")
		return req, false
	}
	return req, true
}

// handleShowInventory reads what a player is carrying.
//
// The game only keeps this once a player has been online for about thirty
// seconds and says so itself when asked sooner, so the reply is passed through
// as written rather than being dressed up as an empty inventory.
func (s *Server) handleShowInventory(w http.ResponseWriter, r *http.Request) {
	entityID, ok := s.entityIDFromPath(w, r)
	if !ok {
		return
	}
	command, err := console.ShowInventory(entityID)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error(), "INVALID_ARGUMENT")
		return
	}
	s.readThrough(w, r, command, "could not read the inventory")
}

func (s *Server) handleUnlockInventories(w http.ResponseWriter, r *http.Request) {
	entityID, ok := s.entityIDFromPath(w, r)
	if !ok {
		return
	}
	command, err := console.UnlockInventories(entityID)
	s.runAction(w, r, command, err)
}

func (s *Server) handleKickAll(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeBody[struct {
		Reason string `json:"reason"`
	}](w, r)
	if !ok {
		return
	}
	command, err := console.KickAll(req.Reason)
	s.runAction(w, r, command, err)
}

func (s *Server) handleListLandClaims(w http.ResponseWriter, r *http.Request) {
	command, err := console.ListLandClaims(r.URL.Query().Get("player"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error(), "INVALID_ARGUMENT")
		return
	}
	s.readThrough(w, r, command, "could not list land claims")
}

func (s *Server) handleRemoveLandClaims(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeBody[platformRequest](w, r)
	if !ok {
		return
	}
	command, err := console.RemoveLandClaims(req.PlatformUserID)
	s.runAction(w, r, command, err)
}

// handleAccess reads the admin and whitelist lists together, because they are
// two halves of one question and nobody wants to ask it twice.
func (s *Server) handleAccess(w http.ResponseWriter, r *http.Request) {
	client := serverFrom(r.Context()).Client
	admins, err := client.Execute(r.Context(), console.ListAdmins())
	if err != nil {
		s.writeGameError(w, err, "could not read the admin list")
		return
	}
	whitelist, err := client.Execute(r.Context(), console.ListWhitelist())
	if err != nil {
		s.writeGameError(w, err, "could not read the whitelist")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{
		"admins":    admins.Result,
		"whitelist": whitelist.Result,
	})
}

func (s *Server) handleSetAdmin(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeBody[struct {
		PlatformUserID string `json:"platformUserId"`
		Level          int    `json:"level"`
	}](w, r)
	if !ok {
		return
	}
	command, err := console.SetAdmin(req.PlatformUserID, req.Level)
	s.runAction(w, r, command, err)
}

func (s *Server) handleRemoveAdmin(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeBody[platformRequest](w, r)
	if !ok {
		return
	}
	command, err := console.RemoveAdmin(req.PlatformUserID)
	s.runAction(w, r, command, err)
}

func (s *Server) handleAddWhitelist(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeBody[platformRequest](w, r)
	if !ok {
		return
	}
	command, err := console.AddToWhitelist(req.PlatformUserID)
	s.runAction(w, r, command, err)
}

func (s *Server) handleRemoveWhitelist(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeBody[platformRequest](w, r)
	if !ok {
		return
	}
	command, err := console.RemoveFromWhitelist(req.PlatformUserID)
	s.runAction(w, r, command, err)
}

func (s *Server) handleSetMaxPlayers(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeBody[struct {
		Count int `json:"count"`
	}](w, r)
	if !ok {
		return
	}
	command, err := console.SetMaxPlayers(req.Count)
	s.runAction(w, r, command, err)
}

func (s *Server) handleShutdown(w http.ResponseWriter, r *http.Request) {
	s.runAction(w, r, console.Shutdown(), nil)
}

// readThrough runs a read-only command and returns its text as written.
func (s *Server) readThrough(w http.ResponseWriter, r *http.Request, command, onFail string) {
	result, err := serverFrom(r.Context()).Client.Execute(r.Context(), command)
	if err != nil {
		s.writeGameError(w, err, onFail)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{
		"command": command,
		"text":    result.Result,
	})
}

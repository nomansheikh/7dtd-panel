package api

import (
	"net/http"
	"time"

	"github.com/nomansheikh/7dtd-panel/internal/console"
	"github.com/nomansheikh/7dtd-panel/internal/httpx"
)

// World actions all reduce to a console command, because the REST API has no
// write path for game state: gameprefs, sandboxsettings and bloodmoon are
// read-only, and PUT returns 405 Unsupported.
//
// Commands are assembled by the builders in internal/console, never by string
// concatenation here, so no request body can smuggle in a second command.

type actionResponse struct {
	// Command is echoed so the operator can see exactly what ran, and paste it
	// into the console themselves if they want to vary it.
	Command string    `json:"command"`
	Result  string    `json:"result"`
	RanAt   time.Time `json:"ranAt"`
}

// run executes a built command and writes the standard action response.
func (s *Server) runAction(w http.ResponseWriter, r *http.Request, command string, buildErr error) {
	if buildErr != nil {
		httpx.WriteError(w, http.StatusBadRequest, buildErr.Error(), "INVALID_ARGUMENT")
		return
	}
	if !console.IsDestructiveAllowed(command, s.cfg.Panel.AllowDestructive) {
		httpx.WriteError(w, http.StatusForbidden,
			"this command is blocked because PANEL_ALLOW_DESTRUCTIVE is false",
			"DESTRUCTIVE_BLOCKED")
		return
	}

	user, _ := UserFrom(r.Context())
	s.log.Info("world action", "username", user.Username, "command", command)

	result, err := serverFrom(r.Context()).Client.Execute(r.Context(), command)

	// Recorded in the same history as the console, so everything an operator
	// did is in one place regardless of which surface they used.
	if _, histErr := s.store.AddCommandRun(r.Context(), commandRun(user.ID, command, result.Result, err, s.now())); histErr != nil {
		s.log.Warn("could not record action history", "error", histErr)
	}

	if err != nil {
		s.writeGameError(w, err, "the action failed")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, actionResponse{
		Command: command,
		Result:  result.Result,
		RanAt:   s.now().UTC(),
	})
}

type setTimeRequest struct {
	Day    int `json:"day"`
	Hour   int `json:"hour"`
	Minute int `json:"minute"`
}

func (s *Server) handleSetTime(w http.ResponseWriter, r *http.Request) {
	var req setTimeRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error(), "INVALID_BODY")
		return
	}
	command, err := console.SetTime(req.Day, req.Hour, req.Minute)
	s.runAction(w, r, command, err)
}

type weatherRequest struct {
	// Setting is one of Clouds, Rain, SnowFall, Wind, Temp, Fog.
	Setting string   `json:"setting"`
	Value   *float64 `json:"value"`
	// Defaults returns the weather to simulated behaviour, ignoring Setting.
	Defaults bool `json:"defaults"`
}

func (s *Server) handleWeather(w http.ResponseWriter, r *http.Request) {
	var req weatherRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error(), "INVALID_BODY")
		return
	}
	if req.Defaults {
		s.runAction(w, r, console.WeatherDefaults(), nil)
		return
	}
	if req.Value == nil {
		httpx.WriteError(w, http.StatusBadRequest,
			"value is required unless defaults is true", "INVALID_ARGUMENT")
		return
	}
	command, err := console.Weather(console.WeatherKnob(req.Setting), *req.Value)
	s.runAction(w, r, command, err)
}

type spawnRequest struct {
	// EntityClass is the name from /api/entities, resolved to its id here so
	// the client never has to know the numeric id.
	EntityClass string `json:"entityClass"`
	X           int    `json:"x"`
	Y           int    `json:"y"`
	Z           int    `json:"z"`
	Count       int    `json:"count"`
}

func (s *Server) handleSpawn(w http.ResponseWriter, r *http.Request) {
	var req spawnRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error(), "INVALID_BODY")
		return
	}

	// Validated against the server's own catalogue, so an unknown name is
	// refused here rather than becoming a command that fails in the game.
	class, found, err := serverFrom(r.Context()).Entities.Lookup(r.Context(), req.EntityClass)
	if err != nil {
		s.writeGameError(w, err, "could not load the entity list")
		return
	}
	if !found {
		httpx.WriteError(w, http.StatusBadRequest,
			"no entity class called "+req.EntityClass+" on this server", "UNKNOWN_ENTITY")
		return
	}
	if class.ManualSpawnType == "None" {
		httpx.WriteError(w, http.StatusBadRequest,
			req.EntityClass+" cannot be spawned manually", "NOT_SPAWNABLE")
		return
	}

	// The command wants the class name, not the id: that id is a hash and the
	// server rejects it. Looking it up first still matters, because it proves
	// the name exists and is spawnable before anything is sent.
	command, buildErr := console.SpawnEntityAt(class.Name, req.X, req.Y, req.Z, req.Count)
	s.runAction(w, r, command, buildErr)
}

type sayRequest struct {
	Message string `json:"message"`
}

func (s *Server) handleSay(w http.ResponseWriter, r *http.Request) {
	var req sayRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error(), "INVALID_BODY")
		return
	}
	command, err := console.Say(req.Message)
	s.runAction(w, r, command, err)
}

// handleWanderingHorde starts a wandering horde.
//
// This is not a blood moon. /api/bloodmoon is read-only and the server has no
// bloodmoon command, so the only way to cause one is to move the clock to its
// day, which the UI offers separately and labels as such.
func (s *Server) handleWanderingHorde(w http.ResponseWriter, r *http.Request) {
	s.runAction(w, r, console.SpawnWanderingHorde(), nil)
}

// handleItems serves a ranked slice of the item catalogue for the picker.
func (s *Server) handleItems(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	items, total, err := serverFrom(r.Context()).Items.Search(
		r.Context(), query.Get("q"), query.Get("blocks") == "true", 50)
	if err != nil {
		s.writeGameError(w, err, "could not load the item list")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"items": items,
		"total": total,
	})
}

// handleEntities serves spawnable entity classes for the picker.
func (s *Server) handleEntities(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	entities, total, err := serverFrom(r.Context()).Entities.Search(
		r.Context(), query.Get("q"), query.Get("all") != "true", 50)
	if err != nil {
		s.writeGameError(w, err, "could not load the entity list")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"entities": entities,
		"total":    total,
	})
}

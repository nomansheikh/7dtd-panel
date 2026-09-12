package api

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/nomansheikh/7dtd-panel/internal/console"
	"github.com/nomansheikh/7dtd-panel/internal/httpx"
	"github.com/nomansheikh/7dtd-panel/internal/sdtd"
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
	// Storm starts a storm in Biome for StormHours, ignoring Setting.
	Storm      bool   `json:"storm"`
	StormHours int    `json:"stormHours"`
	Biome      string `json:"biome"`
}

// handleCurrentWeather reports what the weather is actually doing.
//
// The panel could set weather but never show it, so every change was made
// blind. The bare weather command is the only source; there is no REST
// endpoint for it.
func (s *Server) handleCurrentWeather(w http.ResponseWriter, r *http.Request) {
	weather, err := serverFrom(r.Context()).Client.CurrentWeather(r.Context())
	if err != nil {
		s.writeGameError(w, err, "could not read the weather")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, weather)
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

	if req.Storm {
		// Checked against the biomes the server just reported, because the
		// command answers an unknown name with silence rather than an error.
		weather, err := serverFrom(r.Context()).Client.CurrentWeather(r.Context())
		if err != nil {
			s.writeGameError(w, err, "could not read the weather")
			return
		}
		known := false
		for _, b := range weather.Biomes {
			if b.Biome == req.Biome {
				known = true
				break
			}
		}
		if !known {
			httpx.WriteError(w, http.StatusBadRequest,
				"no biome called "+req.Biome+" in this world", "UNKNOWN_BIOME")
			return
		}
		command, buildErr := console.WeatherStorm(req.StormHours, req.Biome)
		s.runAction(w, r, command, buildErr)
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

func (s *Server) handleAirDrop(w http.ResponseWriter, r *http.Request) {
	s.runAction(w, r, console.SpawnAirDrop(), nil)
}

// handleServerVitals reports how hard the game server is working and what it
// is running. Named apart from /api/health, which is the panel's own liveness.
func (s *Server) handleServerVitals(w http.ResponseWriter, r *http.Request) {
	health, err := serverFrom(r.Context()).Client.ServerHealth(r.Context())
	if err != nil {
		s.writeGameError(w, err, "could not read the server's health")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, health)
}

// handleItemIcon proxies one item's art.
//
// Proxied rather than linked because the browser must never address the game
// server directly: on most installs it is on a private network the panel can
// reach and a laptop cannot, and the panel is the only thing holding the token.
// The icons themselves happen to need no auth, which does not change either.
//
// Cached hard. The art belongs to the game build, so it cannot change while the
// server is up, and a picker showing a hundred of them at once should not mean
// a hundred round trips on every keystroke.
func (s *Server) handleItemIcon(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := console.CheckItemName(name); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error(), "INVALID_ARGUMENT")
		return
	}

	icon, err := serverFrom(r.Context()).Client.ItemIcon(r.Context(), name, r.URL.Query().Get("tint"))
	if errors.Is(err, sdtd.ErrNoIcon) {
		// Ordinary rather than exceptional: plenty of catalogue entries are
		// recipes or internal items the game has never drawn.
		httpx.WriteError(w, http.StatusNotFound, "the game has no icon for that item", "NO_ICON")
		return
	}
	if err != nil {
		s.writeGameError(w, err, "could not load the item icon")
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=604800, immutable")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(icon)
}

// handleSpawnScouts sends screamers to a player.
//
// Takes a player because the bare command is documented as usable only by an
// issuing player, never a remote console. There is no version of this the
// panel can call without a target.
func (s *Server) handleSpawnScouts(w http.ResponseWriter, r *http.Request) {
	var req struct {
		EntityID int `json:"entityId"`
	}
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error(), "INVALID_BODY")
		return
	}
	command, err := console.SpawnScouts(req.EntityID)
	s.runAction(w, r, command, err)
}

// handleSaveWorld writes the world to disk.
func (s *Server) handleSaveWorld(w http.ResponseWriter, r *http.Request) {
	s.runAction(w, r, console.SaveWorld(), nil)
}

// handleKillAll clears entities. Never players, whatever the scope.
func (s *Server) handleKillAll(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Scope string `json:"scope"`
	}
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error(), "INVALID_BODY")
		return
	}
	command, err := console.KillAll(console.KillScope(req.Scope))
	s.runAction(w, r, command, err)
}

// handleResetChunks resets every unprotected chunk in the world.
//
// Destructive, so runAction refuses it unless PANEL_ALLOW_DESTRUCTIVE is set.
func (s *Server) handleResetChunks(w http.ResponseWriter, r *http.Request) {
	s.runAction(w, r, console.ResetChunks(), nil)
}

// handlePrivateMessage sends a message to one player rather than everybody.
func (s *Server) handlePrivateMessage(w http.ResponseWriter, r *http.Request) {
	entityID, ok := s.entityIDFromPath(w, r)
	if !ok {
		return
	}
	var req sayRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error(), "INVALID_BODY")
		return
	}
	command, buildErr := console.SayPlayer(entityID, req.Message)
	s.runAction(w, r, command, buildErr)
}

// handleItems serves a ranked slice of the item catalogue for the picker.
func (s *Server) handleItems(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	items, total, err := serverFrom(r.Context()).Items.Search(
		r.Context(), query.Get("q"), query.Get("blocks") == "true", intParam(query.Get("limit")))
	if err != nil {
		s.writeGameError(w, err, "could not load the item list")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"items": items,
		"total": total,
	})
}

// intParam reads an optional positive integer, leaving the bounds to the
// catalogue rather than duplicating them here. Anything unparseable reads as
// absent, which the catalogue answers with its own default.
func intParam(raw string) int {
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// handleEntities serves spawnable entity classes for the picker.
func (s *Server) handleEntities(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	entities, total, err := serverFrom(r.Context()).Entities.Search(
		r.Context(), query.Get("q"), query.Get("all") != "true", intParam(query.Get("limit")))
	if err != nil {
		s.writeGameError(w, err, "could not load the entity list")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"entities": entities,
		"total":    total,
	})
}

// handleBuffs serves a ranked slice of the buff catalogue for the picker.
func (s *Server) handleBuffs(w http.ResponseWriter, r *http.Request) {
	buffs, total, err := serverFrom(r.Context()).Buffs.Search(
		r.Context(), r.URL.Query().Get("q"), 50)
	if err != nil {
		s.writeGameError(w, err, "could not load the buff list")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"buffs": buffs,
		"total": total,
	})
}

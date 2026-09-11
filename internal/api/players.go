package api

import (
	"net/http"
	"sort"
	"time"

	"github.com/nomansheikh/7dtd-panel/internal/console"
	"github.com/nomansheikh/7dtd-panel/internal/httpx"
	"github.com/nomansheikh/7dtd-panel/internal/sdtd"
)

type playerRow struct {
	EntityID        int    `json:"entityId"`
	Name            string `json:"name"`
	PlatformID      string `json:"platformId"`
	CrossplatformID string `json:"crossplatformId,omitempty"`
	Online          bool   `json:"online"`
	Ping            int    `json:"ping"`
	// Position, level, health and the counters are only known while a player is
	// online; /api/player is the only source and it covers nobody else.
	Position        *sdtd.Position `json:"position,omitempty"`
	Level           int            `json:"level"`
	Health          int            `json:"health"`
	Deaths          int            `json:"deaths"`
	ZombieKills     int            `json:"zombieKills"`
	PlayerKills     int            `json:"playerKills"`
	PlayTimeSeconds int            `json:"playTimeSeconds"`
	LastOnline      *time.Time     `json:"lastOnline,omitempty"`
	Banned          bool           `json:"banned"`
	BanReason       string         `json:"banReason,omitempty"`
	BanUntil        *time.Time     `json:"banUntil,omitempty"`
	IP              string         `json:"ip,omitempty"`
}

// handlePlayers lists everyone the server knows about, online first.
func (s *Server) handlePlayers(w http.ResponseWriter, r *http.Request) {
	players, err := serverFrom(r.Context()).Client.Players(r.Context())
	if err != nil {
		s.writeGameError(w, err, "could not load the player list")
		return
	}

	rows := make([]playerRow, 0, len(players))
	for _, p := range players {
		row := playerRow{
			EntityID: p.EntityID, Name: p.Name,
			PlatformID: p.PlatformID, CrossplatformID: p.CrossplatformID,
			Online: p.Online, Ping: p.Ping, Level: p.Level, Health: p.Health,
			Deaths: p.Deaths, ZombieKills: p.ZombieKills, PlayerKills: p.PlayerKills,
			PlayTimeSeconds: p.PlayTimeSeconds, LastOnline: p.LastOnline,
			Banned: p.Banned, BanReason: p.BanReason, BanUntil: p.BanUntil,
			IP: p.IP,
		}
		if p.Online {
			pos := p.Position
			row.Position = &pos
		}
		rows = append(rows, row)
	}

	// Online first, then most recently seen: the people you can act on now are
	// the ones worth putting at the top.
	sort.SliceStable(rows, func(a, b int) bool {
		if rows[a].Online != rows[b].Online {
			return rows[a].Online
		}
		at, bt := rows[a].LastOnline, rows[b].LastOnline
		switch {
		case at != nil && bt != nil:
			return at.After(*bt)
		case at != nil:
			return true
		case bt != nil:
			return false
		default:
			return rows[a].Name < rows[b].Name
		}
	})

	httpx.WriteJSON(w, http.StatusOK, map[string]any{"players": rows})
}

// Player actions. Each builds its command with a typed builder rather than
// interpolating request fields, and every one that names a player uses the
// entity id: a chosen display name is attacker-controlled, an integer is not.

type teleportRequest struct {
	// Either coordinates or a target player, not both. Declared separately
	// because a shared tag on one line would give all three the same json name.
	X *int `json:"x,omitempty"`
	Y *int `json:"y,omitempty"`
	Z *int `json:"z,omitempty"`
	// ToEntityID moves this player to another one.
	ToEntityID *int `json:"toEntityId,omitempty"`
}

func (s *Server) handleTeleport(w http.ResponseWriter, r *http.Request) {
	entityID, ok := s.entityIDFromPath(w, r)
	if !ok {
		return
	}
	var req teleportRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error(), "INVALID_BODY")
		return
	}

	switch {
	case req.ToEntityID != nil:
		command, err := console.TeleportToPlayer(entityID, *req.ToEntityID)
		s.runAction(w, r, command, err)
	case req.X != nil && req.Y != nil && req.Z != nil:
		command, err := console.Teleport(entityID, *req.X, *req.Y, *req.Z)
		s.runAction(w, r, command, err)
	default:
		httpx.WriteError(w, http.StatusBadRequest,
			"give either coordinates or a player to teleport to", "INVALID_ARGUMENT")
	}
}

type giveRequest struct {
	Item    string `json:"item"`
	Count   int    `json:"count"`
	Quality int    `json:"quality"`
}

func (s *Server) handleGiveItem(w http.ResponseWriter, r *http.Request) {
	entityID, ok := s.entityIDFromPath(w, r)
	if !ok {
		return
	}
	var req giveRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error(), "INVALID_BODY")
		return
	}

	// Checked against the server's own catalogue, so an unknown name is refused
	// here rather than becoming a command that fails in the game.
	known, err := serverFrom(r.Context()).Items.Has(r.Context(), req.Item)
	if err != nil {
		s.writeGameError(w, err, "could not load the item list")
		return
	}
	if !known {
		httpx.WriteError(w, http.StatusBadRequest,
			"no item called "+req.Item+" on this server", "UNKNOWN_ITEM")
		return
	}

	command, buildErr := console.GiveItem(entityID, req.Item, req.Count, req.Quality)
	s.runAction(w, r, command, buildErr)
}

func (s *Server) handleKillPlayer(w http.ResponseWriter, r *http.Request) {
	entityID, ok := s.entityIDFromPath(w, r)
	if !ok {
		return
	}
	command, err := console.Kill(entityID)
	s.runAction(w, r, command, err)
}

type kickRequest struct {
	Reason string `json:"reason"`
}

func (s *Server) handleKick(w http.ResponseWriter, r *http.Request) {
	entityID, ok := s.entityIDFromPath(w, r)
	if !ok {
		return
	}
	var req kickRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error(), "INVALID_BODY")
		return
	}
	command, buildErr := console.Kick(entityID, req.Reason)
	s.runAction(w, r, command, buildErr)
}

type banRequest struct {
	// PlatformID rather than entity id: ban is the one action that must work
	// for someone who has already left.
	PlatformID string `json:"platformId"`
	Duration   int    `json:"duration"`
	Unit       string `json:"unit"`
	Reason     string `json:"reason"`
}

func (s *Server) handleBan(w http.ResponseWriter, r *http.Request) {
	var req banRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error(), "INVALID_BODY")
		return
	}
	command, buildErr := console.Ban(req.PlatformID, req.Duration,
		console.BanUnit(req.Unit), req.Reason)
	s.runAction(w, r, command, buildErr)
}

func (s *Server) handleUnban(w http.ResponseWriter, r *http.Request) {
	var req banRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error(), "INVALID_BODY")
		return
	}
	command, buildErr := console.Unban(req.PlatformID)
	s.runAction(w, r, command, buildErr)
}

type buffRequest struct {
	Buff string `json:"buff"`
	// Remove debuffs instead of applying.
	Remove bool `json:"remove"`
}

func (s *Server) handleBuff(w http.ResponseWriter, r *http.Request) {
	entityID, ok := s.entityIDFromPath(w, r)
	if !ok {
		return
	}
	var req buffRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error(), "INVALID_BODY")
		return
	}
	var (
		command  string
		buildErr error
	)
	if req.Remove {
		command, buildErr = console.Debuff(entityID, req.Buff)
	} else {
		command, buildErr = console.Buff(entityID, req.Buff)
	}
	s.runAction(w, r, command, buildErr)
}

type xpRequest struct {
	Amount int `json:"amount"`
}

// handleGiveXP grants experience.
//
// There is no command to set a level: the game exposes only additive XP, so a
// level cannot be lowered or set precisely. The UI says so.
func (s *Server) handleGiveXP(w http.ResponseWriter, r *http.Request) {
	entityID, ok := s.entityIDFromPath(w, r)
	if !ok {
		return
	}
	var req xpRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error(), "INVALID_BODY")
		return
	}
	command, buildErr := console.GiveXP(entityID, req.Amount)
	s.runAction(w, r, command, buildErr)
}

// entityIDFromPath parses the {entityId} segment.
func (s *Server) entityIDFromPath(w http.ResponseWriter, r *http.Request) (int, bool) {
	raw := r.PathValue("entityId")
	id, err := parsePositiveInt(raw)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest,
			"entity id must be a non-negative number, got "+raw, "INVALID_ENTITY_ID")
		return 0, false
	}
	return id, true
}

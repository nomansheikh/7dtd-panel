package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/nomansheikh/7dtd-panel/internal/chat"
	"github.com/nomansheikh/7dtd-panel/internal/httpx"
	"github.com/nomansheikh/7dtd-panel/internal/store"
)

/*
The chat bot's configuration, and the kits it hands out.

The command list is fixed in the chat package, so this never creates a command
— it only says, for each one the panel knows how to answer, whether it is on,
who may run it and how often. An operator cannot add a command here, which is
the point: the set of things a stranger in chat can make the panel do is decided
at compile time, not by a row in a database.
*/

// chatCommandRow is one command as the UI sees it: what it does, joined to how
// it is configured.
type chatCommandRow struct {
	Name    string `json:"name"`
	Usage   string `json:"usage"`
	Summary string `json:"summary"`
	// Acts marks the one command that changes game state, so the UI can warn
	// before it is opened to everyone.
	Acts            bool           `json:"acts"`
	Enabled         bool           `json:"enabled"`
	Audience        store.Audience `json:"audience"`
	CooldownSeconds int            `json:"cooldownSeconds"`
}

// handleChatCommands lists every command with its configuration.
//
// Commands with no row are returned as switched off with the defaults the panel
// suggests, rather than being absent: the UI shows what could be turned on, and
// a command nobody has configured is exactly what an operator is looking for.
func (s *Server) handleChatCommands(w http.ResponseWriter, r *http.Request) {
	srv := serverFrom(r.Context())

	configured, err := s.store.ChatCommands(r.Context(), srv.ID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError,
			"could not read the chat commands", "STORE_ERROR")
		return
	}
	byName := make(map[string]store.ChatCommand, len(configured))
	for _, row := range configured {
		byName[row.Name] = row
	}

	specs := chat.Specs()
	rows := make([]chatCommandRow, 0, len(specs))
	for _, spec := range specs {
		row := chatCommandRow{
			Name:            spec.Name,
			Usage:           spec.Usage,
			Summary:         spec.Summary,
			Acts:            spec.Acts,
			Audience:        store.AudienceEveryone,
			CooldownSeconds: spec.DefaultCooldownSeconds,
		}
		if cfg, ok := byName[spec.Name]; ok {
			row.Enabled = cfg.Enabled
			row.Audience = cfg.Audience
			row.CooldownSeconds = cfg.CooldownSeconds
		}
		rows = append(rows, row)
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"prefix":   chat.Prefix,
		"commands": rows,
	})
}

// maxCooldownSeconds is a week, which is longer than any cooldown that makes
// sense and short enough that a typo cannot disable a command by accident for
// the life of the world.
const maxCooldownSeconds = 7 * 24 * 60 * 60

// handleSaveChatCommand configures one command.
func (s *Server) handleSaveChatCommand(w http.ResponseWriter, r *http.Request) {
	srv := serverFrom(r.Context())
	name := strings.ToLower(r.PathValue("name"))

	// The name comes from the path, and only a command the panel actually
	// implements may be written. Otherwise the table fills with rows that
	// configure nothing and look like they configure something.
	var known bool
	for _, spec := range chat.Specs() {
		if spec.Name == name {
			known = true
			break
		}
	}
	if !known {
		httpx.WriteError(w, http.StatusNotFound, "no such chat command", "NOT_FOUND")
		return
	}

	req, ok := decodeBody[struct {
		Enabled         bool   `json:"enabled"`
		Audience        string `json:"audience"`
		CooldownSeconds int    `json:"cooldownSeconds"`
	}](w, r)
	if !ok {
		return
	}

	audience := store.Audience(req.Audience)
	if audience != store.AudienceEveryone && audience != store.AudienceAdmins {
		httpx.WriteError(w, http.StatusBadRequest,
			`audience must be "everyone" or "admins"`, "INVALID_ARGUMENT")
		return
	}
	if req.CooldownSeconds < 0 || req.CooldownSeconds > maxCooldownSeconds {
		httpx.WriteError(w, http.StatusBadRequest,
			"cooldown must be between 0 seconds and a week", "INVALID_ARGUMENT")
		return
	}

	err := s.store.SaveChatCommand(r.Context(), srv.ID, store.ChatCommand{
		Name:            name,
		Enabled:         req.Enabled,
		Audience:        audience,
		CooldownSeconds: req.CooldownSeconds,
	}, s.now())
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError,
			"could not save the chat command", "STORE_ERROR")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

/* ------------------------------------------------------------------ kits -- */

// kitName is what a kit may be called. The same shape the bot accepts, so a kit
// that can be saved is always a kit that can be asked for by name in chat.
var kitName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,31}$`)

// maxKitItems bounds a kit. It mirrors the bot's own limit: a kit it would
// refuse to hand over is not worth being able to save.
const maxKitItems = 25

type kitRow struct {
	Name  string         `json:"name"`
	Items []chat.KitItem `json:"items"`
}

// handleKits lists every saved kit.
//
// Kits are panel-wide rather than per server: a kit is a list of item names,
// and the items belong to the game rather than to one world.
func (s *Server) handleKits(w http.ResponseWriter, r *http.Request) {
	kits, err := s.store.Kits(r.Context())
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "could not read the kits", "STORE_ERROR")
		return
	}
	rows := make([]kitRow, 0, len(kits))
	for _, kit := range kits {
		var items []chat.KitItem
		if err := json.Unmarshal([]byte(kit.Items), &items); err != nil {
			// A kit that cannot be read is still worth listing, so an operator
			// can see it and delete it rather than wondering where it went.
			s.log.Warn("stored kit is not readable", "kit", kit.Name, "error", err)
		}
		rows = append(rows, kitRow{Name: kit.Name, Items: items})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"kits": rows})
}

// handleSaveKit creates or replaces a kit.
//
// Item names are checked against the server's own catalogue here rather than
// when the kit is handed over. A kit with a typo in it should fail at the
// keyboard of the person who wrote it, not at three in the morning in front of
// the player who asked for it.
func (s *Server) handleSaveKit(w http.ResponseWriter, r *http.Request) {
	srv := serverFrom(r.Context())
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	if !kitName.MatchString(name) {
		httpx.WriteError(w, http.StatusBadRequest,
			"a kit name may use lowercase letters, digits, dash and underscore, up to 32 characters",
			"INVALID_ARGUMENT")
		return
	}

	req, ok := decodeBody[struct {
		Items []chat.KitItem `json:"items"`
	}](w, r)
	if !ok {
		return
	}
	if len(req.Items) == 0 {
		httpx.WriteError(w, http.StatusBadRequest, "a kit needs at least one item", "INVALID_ARGUMENT")
		return
	}
	if len(req.Items) > maxKitItems {
		httpx.WriteError(w, http.StatusBadRequest,
			"a kit may hold up to 25 different items", "INVALID_ARGUMENT")
		return
	}

	for i := range req.Items {
		if req.Items[i].Count < 1 {
			req.Items[i].Count = 1
		}
		if req.Items[i].Quality < 0 {
			req.Items[i].Quality = 0
		}
		known, err := srv.Items.Has(r.Context(), req.Items[i].Item)
		if err != nil {
			s.writeGameError(w, err, "could not check the item list")
			return
		}
		if !known {
			httpx.WriteError(w, http.StatusBadRequest,
				"the server does not have an item called "+req.Items[i].Item, "UNKNOWN_ITEM")
			return
		}
	}

	encoded, err := json.Marshal(req.Items)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "the kit could not be encoded", "INVALID_ARGUMENT")
		return
	}
	if err := s.store.SaveKit(r.Context(), name, string(encoded), s.now()); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "could not save the kit", "STORE_ERROR")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, kitRow{Name: name, Items: req.Items})
}

func (s *Server) handleDeleteKit(w http.ResponseWriter, r *http.Request) {
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	err := s.store.DeleteKit(r.Context(), name)
	if errors.Is(err, store.ErrNotFound) {
		httpx.WriteError(w, http.StatusNotFound, "no kit by that name", "NOT_FOUND")
		return
	}
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "could not delete the kit", "STORE_ERROR")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

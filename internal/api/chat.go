package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/nomansheikh/7dtd-panel/internal/chat"
	"github.com/nomansheikh/7dtd-panel/internal/console"
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

// commandLine is one console line of a custom command, with what the panel
// already knows about it.
//
// The tier and whether it is blocked come from the same policy the console page
// uses. They are reported rather than enforced here so the UI can show an admin
// what they are about to hand to players before they hand it over.
type commandLine struct {
	Line string `json:"line"`
	Tier string `json:"tier"`
	// Blocked is true when this line is destructive and PANEL_ALLOW_DESTRUCTIVE
	// is off, which is the operator's own switch rather than the panel's
	// opinion.
	Blocked bool `json:"blocked"`
	// Problem is set when the line would be refused outright, so a typo is
	// caught at the keyboard.
	Problem string `json:"problem,omitempty"`
}

// chatCommandRow is one command as the UI sees it: what it does, joined to how
// it is configured.
type chatCommandRow struct {
	Name string `json:"name"`
	// Kind is "builtin" for the commands the panel ships with, "custom" for the
	// ones an admin wrote. Only a custom one can be edited or deleted.
	Kind    store.Kind `json:"kind"`
	Usage   string     `json:"usage"`
	Summary string     `json:"summary"`
	// Acts marks a command that changes game state rather than reporting it, so
	// the UI can say so before it is opened to everyone.
	Acts            bool           `json:"acts"`
	Enabled         bool           `json:"enabled"`
	Audience        store.Audience `json:"audience"`
	CooldownSeconds int            `json:"cooldownSeconds"`

	// Reply and Commands are the behaviour of a custom command. Empty for a
	// built-in, whose behaviour is in code.
	Reply    string        `json:"reply,omitempty"`
	Commands []commandLine `json:"commands,omitempty"`
	// Tier is the strongest tier of any of its lines, which is what decides how
	// loudly the panel talks about it.
	Tier string `json:"tier,omitempty"`
}

// describe annotates one console line.
func (s *Server) describe(line string) commandLine {
	out := commandLine{Line: line, Tier: string(console.ClassifyTier(line))}
	if err := console.Validate(line); err != nil {
		out.Problem = err.Error()
		return out
	}
	out.Blocked = !console.IsDestructiveAllowed(line, s.cfg.Panel.AllowDestructive)
	return out
}

// strongestTier is the one an admin should be told about.
func strongestTier(lines []commandLine) string {
	tier := string(console.TierNormal)
	for _, line := range lines {
		switch console.Tier(line.Tier) {
		case console.TierDestructive:
			return string(console.TierDestructive)
		case console.TierMutating:
			tier = string(console.TierMutating)
		}
	}
	return tier
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
	rows := make([]chatCommandRow, 0, len(specs)+len(configured))
	for _, spec := range specs {
		row := chatCommandRow{
			Name:            spec.Name,
			Kind:            store.KindBuiltin,
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

	// Then the admin's own, in the order the store returned them, which is by
	// name. They come after the built-ins because the built-ins are the ones
	// that are always there.
	for _, cfg := range configured {
		if cfg.Kind != store.KindCustom {
			continue
		}
		lines := make([]commandLine, 0, len(cfg.Commands))
		for _, line := range cfg.Commands {
			lines = append(lines, s.describe(line))
		}
		rows = append(rows, chatCommandRow{
			Name:            cfg.Name,
			Kind:            store.KindCustom,
			Usage:           chat.Prefix + cfg.Name,
			Summary:         cfg.Description,
			Acts:            len(cfg.Commands) > 0,
			Enabled:         cfg.Enabled,
			Audience:        cfg.Audience,
			CooldownSeconds: cfg.CooldownSeconds,
			Reply:           cfg.Reply,
			Commands:        lines,
			Tier:            strongestTier(lines),
		})
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"prefix":       chat.Prefix,
		"commands":     rows,
		"placeholders": chat.Placeholders,
		// So the UI can say why a destructive line will not run, rather than
		// leaving an admin to find out from a player.
		"allowDestructive": s.cfg.Panel.AllowDestructive,
	})
}

// maxCooldownSeconds is a week, which is longer than any cooldown that makes
// sense and short enough that a typo cannot disable a command by accident for
// the life of the world.
const maxCooldownSeconds = 7 * 24 * 60 * 60

// commandName is what a command may be called: typeable in chat, and unable to
// be mistaken for a second console argument.
var commandName = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,23}$`)

// validateLine is the check both a custom chat command and a task put their
// lines through: single line, no nulls, nothing that smuggles in a second
// command. The same one the console page runs.
func validateLine(line string) error { return console.Validate(line) }

// maxCommandLines bounds one custom command. Long enough for a starter package
// that hands over a dozen things, short enough that one chat message cannot
// become an unbounded run of console commands.
const maxCommandLines = 25

/*
handleSaveChatCommand configures a built-in, or creates and edits one of the
admin's own.

Which of those it is comes from the name: a built-in's name is reserved, so
writing to it configures the built-in and cannot replace it. Anything else is a
custom command, created on first write.

There is deliberately no allowlist of what a custom command may run. The person
writing it already has a console page in this panel that runs anything they
type, and the tier of every line is reported back so they can see what they are
switching on. PANEL_ALLOW_DESTRUCTIVE still applies, because that is their own
switch and a chat trigger is not a way around a setting they chose.
*/
func (s *Server) handleSaveChatCommand(w http.ResponseWriter, r *http.Request) {
	srv := serverFrom(r.Context())
	name := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(r.PathValue("name")), chat.Prefix))

	builtin := false
	for _, spec := range chat.Specs() {
		if spec.Name == name {
			builtin = true
			break
		}
	}
	if !builtin && !commandName.MatchString(name) {
		httpx.WriteError(w, http.StatusBadRequest,
			"a command name may use lowercase letters, digits, dash and underscore, "+
				"start with a letter, and run to 24 characters", "INVALID_ARGUMENT")
		return
	}

	req, ok := decodeBody[struct {
		Enabled         bool     `json:"enabled"`
		Audience        string   `json:"audience"`
		CooldownSeconds int      `json:"cooldownSeconds"`
		Description     string   `json:"description"`
		Reply           string   `json:"reply"`
		Commands        []string `json:"commands"`
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

	saved := store.ChatCommand{
		Name:            name,
		Kind:            store.KindBuiltin,
		Enabled:         req.Enabled,
		Audience:        audience,
		CooldownSeconds: req.CooldownSeconds,
	}

	if !builtin {
		lines := make([]string, 0, len(req.Commands))
		for _, line := range req.Commands {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			// Checked here so a typo fails at the keyboard of the person who
			// made it, rather than in front of the player who triggered it.
			// This is the same check the console page runs: single line, no
			// nulls, nothing that smuggles in a second command.
			if err := validateLine(line); err != nil {
				httpx.WriteError(w, http.StatusBadRequest, err.Error(), "INVALID_COMMAND")
				return
			}
			lines = append(lines, line)
		}
		if len(lines) > maxCommandLines {
			httpx.WriteError(w, http.StatusBadRequest,
				"a command may run up to 25 lines", "INVALID_ARGUMENT")
			return
		}
		if strings.TrimSpace(req.Reply) == "" && len(lines) == 0 {
			httpx.WriteError(w, http.StatusBadRequest,
				"give it something to say, something to run, or both", "INVALID_ARGUMENT")
			return
		}

		saved.Kind = store.KindCustom
		saved.Description = strings.TrimSpace(req.Description)
		saved.Reply = strings.TrimSpace(req.Reply)
		saved.Commands = lines
	}

	if err := s.store.SaveChatCommand(r.Context(), srv.ID, saved, s.now()); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError,
			"could not save the chat command", "STORE_ERROR")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleDeleteChatCommand removes a command an admin wrote.
//
// A built-in cannot be deleted, only switched off: it would come back on the
// next read, since its existence is in the code rather than the table.
func (s *Server) handleDeleteChatCommand(w http.ResponseWriter, r *http.Request) {
	srv := serverFrom(r.Context())
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))

	for _, spec := range chat.Specs() {
		if spec.Name == name {
			httpx.WriteError(w, http.StatusBadRequest,
				"that command is built in; switch it off instead", "BUILTIN_COMMAND")
			return
		}
	}

	err := s.store.DeleteChatCommand(r.Context(), srv.ID, name)
	if errors.Is(err, store.ErrNotFound) {
		httpx.WriteError(w, http.StatusNotFound, "no command by that name", "NOT_FOUND")
		return
	}
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError,
			"could not delete the chat command", "STORE_ERROR")
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

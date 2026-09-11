package api

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/nomansheikh/7dtd-panel/internal/console"
	"github.com/nomansheikh/7dtd-panel/internal/httpx"
	"github.com/nomansheikh/7dtd-panel/internal/sdtd"
	"github.com/nomansheikh/7dtd-panel/internal/store"
)

type commandInfo struct {
	Name        string   `json:"name"`
	Aliases     []string `json:"aliases"`
	Description string   `json:"description"`
	Help        string   `json:"help,omitempty"`
	Allowed     bool     `json:"allowed"`
	// Tier is the panel's own classification, driving how much confirmation the
	// UI demands. It is not something the game server reports.
	Tier string `json:"tier"`
	// Blocked is true when PANEL_ALLOW_DESTRUCTIVE is off and this command is
	// destructive.
	Blocked bool `json:"blocked"`
}

type commandsResponse struct {
	Commands []commandInfo `json:"commands"`
	// FetchedAt lets the UI show how fresh the catalogue is; it is cached.
	FetchedAt time.Time `json:"fetchedAt"`
}

// handleConsoleCommands serves the server's own command catalogue.
//
// The palette is built from this rather than a hardcoded list, so it reflects
// what this server actually accepts, including commands added by mods, and
// carries the server's own help text.
func (s *Server) handleConsoleCommands(w http.ResponseWriter, r *http.Request) {
	items, fetchedAt, err := s.commands.Get(r.Context())
	if err != nil {
		s.writeGameError(w, err, "could not load the command list")
		return
	}

	out := make([]commandInfo, 0, len(items))
	for _, item := range items {
		tier := console.ClassifyTier(item.Command)
		info := commandInfo{
			Name:        item.Command,
			Aliases:     item.Overloads,
			Description: item.Description,
			Allowed:     item.Allowed == nil || *item.Allowed,
			Tier:        string(tier),
			Blocked:     tier == console.TierDestructive && !s.cfg.Panel.AllowDestructive,
		}
		if item.Help != nil {
			info.Help = *item.Help
		}
		out = append(out, info)
	}

	httpx.WriteJSON(w, http.StatusOK, commandsResponse{Commands: out, FetchedAt: fetchedAt})
}

type executeRequest struct {
	Command string `json:"command"`
}

type executeResponse struct {
	Command    string    `json:"command"`
	Parameters string    `json:"parameters"`
	Result     string    `json:"result"`
	Tier       string    `json:"tier"`
	RanAt      time.Time `json:"ranAt"`
}

// handleConsoleExecute runs a console command.
//
// This is the only write path for game state: gameprefs, sandboxsettings and
// bloodmoon are all read-only over REST.
func (s *Server) handleConsoleExecute(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFrom(r.Context())

	var req executeRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error(), "INVALID_BODY")
		return
	}
	if err := console.Validate(req.Command); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error(), "INVALID_COMMAND")
		return
	}

	tier := console.ClassifyTier(req.Command)
	if !console.IsDestructiveAllowed(req.Command, s.cfg.Panel.AllowDestructive) {
		httpx.WriteError(w, http.StatusForbidden,
			"this command is blocked because PANEL_ALLOW_DESTRUCTIVE is false",
			"DESTRUCTIVE_BLOCKED")
		return
	}

	// Logged before running, so an operator-issued command is attributable even
	// if the server never answers.
	s.log.Info("console command",
		"username", user.Username, "command", req.Command, "tier", string(tier))

	result, execErr := s.game.Execute(r.Context(), req.Command)

	run := store.CommandRun{
		UserID:    user.ID,
		Command:   req.Command,
		Succeeded: execErr == nil,
		CreatedAt: s.now().UTC(),
	}
	if execErr != nil {
		run.Error = gameErrorMessage(execErr)
	} else {
		run.Result = result.Result
	}
	if _, err := s.store.AddCommandRun(r.Context(), run); err != nil {
		// History is a convenience; failing to record it should not fail the
		// command that already ran.
		s.log.Warn("could not record command history", "error", err)
	}

	if execErr != nil {
		s.writeGameError(w, execErr, "the command failed")
		return
	}

	httpx.WriteJSON(w, http.StatusOK, executeResponse{
		Command:    result.Command,
		Parameters: result.Parameters,
		Result:     result.Result,
		Tier:       string(tier),
		RanAt:      run.CreatedAt,
	})
}

type historyEntry struct {
	ID        int64     `json:"id"`
	Command   string    `json:"command"`
	Succeeded bool      `json:"succeeded"`
	Result    string    `json:"result,omitempty"`
	Error     string    `json:"error,omitempty"`
	RanAt     time.Time `json:"ranAt"`
}

// handleConsoleHistory returns this operator's recent commands, newest first.
func (s *Server) handleConsoleHistory(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFrom(r.Context())

	limit := 100
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			httpx.WriteError(w, http.StatusBadRequest,
				"limit must be a positive number", "INVALID_LIMIT")
			return
		}
		limit = n
	}

	runs, err := s.store.RecentCommandRuns(r.Context(), user.ID, limit)
	if err != nil {
		s.log.Error("could not read command history", "error", err)
		httpx.WriteError(w, http.StatusInternalServerError,
			"could not read the command history", "STORE_ERROR")
		return
	}

	out := make([]historyEntry, 0, len(runs))
	for _, run := range runs {
		out = append(out, historyEntry{
			ID: run.ID, Command: run.Command, Succeeded: run.Succeeded,
			Result: run.Result, Error: run.Error, RanAt: run.CreatedAt,
		})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"history": out})
}

// gameErrorMessage renders a game server failure for an operator, keeping the
// server's own wording and dropping the stack trace.
func gameErrorMessage(err error) string {
	var apiErr *sdtd.APIError
	if errors.As(err, &apiErr) {
		return apiErr.SafeMessage()
	}
	return err.Error()
}

// writeGameError relays a game server failure with its real message and code.
//
// The brief is explicit that the operator should see what actually went wrong
// rather than a generic failure, so the server's own errorCode is passed
// through for the UI to branch on.
func (s *Server) writeGameError(w http.ResponseWriter, err error, context string) {
	var apiErr *sdtd.APIError
	if errors.As(err, &apiErr) {
		status := http.StatusBadGateway
		switch apiErr.Status {
		case http.StatusNotFound, http.StatusBadRequest, http.StatusForbidden:
			status = apiErr.Status
		}
		// The trace goes to the panel's log, never to the browser.
		if apiErr.Trace != "" {
			s.log.Error("game server exception",
				"code", apiErr.ErrorCode, "message", apiErr.ExceptionMessage,
				"trace", apiErr.Trace)
		}
		httpx.WriteError(w, status, apiErr.SafeMessage(), apiErr.ErrorCode)
		return
	}
	s.log.Warn(context, "error", err)
	httpx.WriteError(w, http.StatusBadGateway, context+": "+err.Error(), "GAME_UNREACHABLE")
}

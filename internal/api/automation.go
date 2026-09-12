package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/nomansheikh/7dtd-panel/internal/automation"
	"github.com/nomansheikh/7dtd-panel/internal/httpx"
	"github.com/nomansheikh/7dtd-panel/internal/store"
)

/*
Tasks the panel carries out on its own.

Deliberately the same shape as a custom chat command — a name, console lines,
and a switch — because they are the same act with a different thing setting
them off. The lines are reported back with their tier for the same reason: an
operator switching on a nightly shutdown should see that it is a shutdown.
*/

type taskRow struct {
	Name        string            `json:"name"`
	Enabled     bool              `json:"enabled"`
	Description string            `json:"description,omitempty"`
	Trigger     store.TriggerKind `json:"trigger"`
	Minutes     int               `json:"minutes,omitempty"`
	At          string            `json:"at,omitempty"`
	Commands    []commandLine     `json:"commands"`
	Tier        string            `json:"tier"`
	LastRunAt   string            `json:"lastRunAt,omitempty"`
}

func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	srv := serverFrom(r.Context())

	tasks, err := s.store.Tasks(r.Context(), srv.ID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "could not read the tasks", "STORE_ERROR")
		return
	}

	rows := make([]taskRow, 0, len(tasks))
	for _, t := range tasks {
		lines := make([]commandLine, 0, len(t.Commands))
		for _, line := range t.Commands {
			lines = append(lines, s.describe(line))
		}
		row := taskRow{
			Name:        t.Name,
			Enabled:     t.Enabled,
			Description: t.Description,
			Trigger:     t.Trigger,
			Minutes:     t.Minutes,
			At:          t.At,
			Commands:    lines,
			Tier:        strongestTier(lines),
		}
		if !t.LastRunAt.IsZero() {
			row.LastRunAt = t.LastRunAt.Format("2006-01-02T15:04:05Z07:00")
		}
		rows = append(rows, row)
	}

	runs, err := s.store.RecentTaskRuns(r.Context(), srv.ID, 20)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "could not read the history", "STORE_ERROR")
		return
	}
	if runs == nil {
		// A nil slice marshals to null, and the browser is promised a list. An
		// empty history is a list of nothing, not the absence of a history.
		runs = []store.TaskRun{}
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"tasks":            rows,
		"runs":             runs,
		"allowDestructive": s.cfg.Panel.AllowDestructive,
	})
}

func (s *Server) handleSaveTask(w http.ResponseWriter, r *http.Request) {
	srv := serverFrom(r.Context())
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))
	if !commandName.MatchString(name) {
		httpx.WriteError(w, http.StatusBadRequest,
			"a task name may use lowercase letters, digits, dash and underscore, "+
				"start with a letter, and run to 24 characters", "INVALID_ARGUMENT")
		return
	}

	req, ok := decodeBody[struct {
		Enabled     bool     `json:"enabled"`
		Description string   `json:"description"`
		Trigger     string   `json:"trigger"`
		Minutes     int      `json:"minutes"`
		At          string   `json:"at"`
		Commands    []string `json:"commands"`
	}](w, r)
	if !ok {
		return
	}

	lines := make([]string, 0, len(req.Commands))
	for _, line := range req.Commands {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Checked at the keyboard of the person writing it, rather than at five
		// in the morning when nobody is watching.
		if err := validateLine(line); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, err.Error(), "INVALID_COMMAND")
			return
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		httpx.WriteError(w, http.StatusBadRequest,
			"a task needs at least one command to run", "INVALID_ARGUMENT")
		return
	}
	if len(lines) > maxCommandLines {
		httpx.WriteError(w, http.StatusBadRequest,
			"a task may run up to 25 lines", "INVALID_ARGUMENT")
		return
	}

	task := store.Task{
		Name:        name,
		Enabled:     req.Enabled,
		Description: strings.TrimSpace(req.Description),
		Trigger:     store.TriggerKind(req.Trigger),
		Minutes:     req.Minutes,
		At:          strings.TrimSpace(req.At),
		Commands:    lines,
	}
	if err := automation.ValidateTrigger(task); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error(), "INVALID_TRIGGER")
		return
	}

	if err := s.store.SaveTask(r.Context(), srv.ID, task, s.now()); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "could not save the task", "STORE_ERROR")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleDeleteTask(w http.ResponseWriter, r *http.Request) {
	srv := serverFrom(r.Context())
	name := strings.ToLower(strings.TrimSpace(r.PathValue("name")))

	err := s.store.DeleteTask(r.Context(), srv.ID, name)
	if errors.Is(err, store.ErrNotFound) {
		httpx.WriteError(w, http.StatusNotFound, "no task by that name", "NOT_FOUND")
		return
	}
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "could not delete the task", "STORE_ERROR")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// TriggerKind is what sets a task off.
type TriggerKind string

const (
	// TriggerEvery fires on a real-time interval.
	TriggerEvery TriggerKind = "every"
	// TriggerDaily fires once a day at a wall-clock time.
	TriggerDaily TriggerKind = "daily"
	// TriggerBloodmoon fires a set time before the horde arrives, which is the
	// one no ordinary scheduler can express: it moves with the game's clock,
	// not the wall's.
	TriggerBloodmoon TriggerKind = "bloodmoon"
	// TriggerJoin fires when a player connects.
	TriggerJoin TriggerKind = "join"
)

// Task is one thing the panel does on its own.
type Task struct {
	Name        string      `json:"name"`
	Enabled     bool        `json:"enabled"`
	Description string      `json:"description,omitempty"`
	Trigger     TriggerKind `json:"trigger"`
	// Minutes is the interval for every, and how long before for bloodmoon.
	Minutes int `json:"minutes,omitempty"`
	// At is "HH:MM" for daily, in the panel's own timezone.
	At       string   `json:"at,omitempty"`
	Commands []string `json:"commands"`

	// LastRunAt and LastKey are the bookkeeping that stops a restart
	// re-running this morning's task, or firing once a poll for half an hour.
	LastRunAt time.Time `json:"lastRunAt,omitzero"`
	LastKey   string    `json:"-"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// TaskRun is one record of a task going off.
type TaskRun struct {
	Name  string    `json:"name"`
	RanAt time.Time `json:"ranAt"`
	Error string    `json:"error,omitempty"`
}

// Tasks lists one server's tasks, in name order.
func (s *Store) Tasks(ctx context.Context, serverID string) ([]Task, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT name, enabled, description, trigger_kind, trigger_minutes, trigger_at,
		        commands, last_run_at, last_key, updated_at
		   FROM automation_tasks WHERE server_id = ? ORDER BY name`, serverID)
	if err != nil {
		return nil, fmt.Errorf("store: list tasks: %w", err)
	}
	defer rows.Close()

	var out []Task
	for rows.Next() {
		var t Task
		var enabled int
		var commands string
		var lastRun, updated int64
		if err := rows.Scan(&t.Name, &enabled, &t.Description, &t.Trigger, &t.Minutes,
			&t.At, &commands, &lastRun, &t.LastKey, &updated); err != nil {
			return nil, fmt.Errorf("store: scan task: %w", err)
		}
		t.Enabled = enabled == 1
		if lastRun > 0 {
			t.LastRunAt = time.Unix(lastRun, 0).UTC()
		}
		t.UpdatedAt = time.Unix(updated, 0).UTC()
		if err := json.Unmarshal([]byte(commands), &t.Commands); err != nil {
			t.Commands = nil
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// SaveTask writes a task, leaving its run bookkeeping alone.
//
// Editing a daily task at noon must not make it run again at noon: the last-run
// time belongs to the running, not to the writing, so it is never touched here.
func (s *Store) SaveTask(ctx context.Context, serverID string, t Task, now time.Time) error {
	commands := t.Commands
	if commands == nil {
		commands = []string{}
	}
	encoded, err := json.Marshal(commands)
	if err != nil {
		return fmt.Errorf("store: encode task commands: %w", err)
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO automation_tasks
		   (server_id, name, enabled, description, trigger_kind, trigger_minutes,
		    trigger_at, commands, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT (server_id, name) DO UPDATE SET
		   enabled = excluded.enabled,
		   description = excluded.description,
		   trigger_kind = excluded.trigger_kind,
		   trigger_minutes = excluded.trigger_minutes,
		   trigger_at = excluded.trigger_at,
		   commands = excluded.commands,
		   updated_at = excluded.updated_at`,
		serverID, t.Name, boolToInt(t.Enabled), t.Description, string(t.Trigger),
		t.Minutes, t.At, string(encoded), now.Unix())
	if err != nil {
		return fmt.Errorf("store: save task: %w", err)
	}
	return nil
}

// DeleteTask removes a task and the record of what it did.
func (s *Store) DeleteTask(ctx context.Context, serverID, name string) error {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM automation_tasks WHERE server_id = ? AND name = ?`, serverID, name)
	if err != nil {
		return fmt.Errorf("store: delete task: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	if _, err := s.db.ExecContext(ctx,
		`DELETE FROM automation_runs WHERE server_id = ? AND name = ?`, serverID, name); err != nil {
		return fmt.Errorf("store: delete task history for %s: %w", name, err)
	}
	return nil
}

/*
ClaimTask records that a task is running, and reports whether this caller is
the one that got it.

The claim is the same insert-decides pattern the chat cooldowns use, for the
same reason: two pollers a moment apart must not both decide a daily task is
due. The update only lands when the stored bookkeeping still looks the way the
caller believed it did, so exactly one of them writes and exactly one of them
runs the commands.
*/
func (s *Store) ClaimTask(
	ctx context.Context, serverID, name string,
	expect Task, newKey string, now time.Time,
) (bool, error) {
	res, err := s.db.ExecContext(ctx,
		`UPDATE automation_tasks SET last_run_at = ?, last_key = ?
		   WHERE server_id = ? AND name = ? AND last_run_at = ? AND last_key = ?`,
		now.Unix(), newKey, serverID, name, unixOrZero(expect.LastRunAt), expect.LastKey)
	if err != nil {
		return false, fmt.Errorf("store: claim task: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("store: claim task: %w", err)
	}
	return n > 0, nil
}

// unixOrZero matches how a never-run task is stored, which is 0 rather than the
// Unix epoch.
func unixOrZero(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.Unix()
}

// AddTaskRun records that a task went off, and what came of it.
func (s *Store) AddTaskRun(ctx context.Context, serverID string, run TaskRun) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO automation_runs (server_id, name, ran_at, error) VALUES (?, ?, ?, ?)`,
		serverID, run.Name, run.RanAt.Unix(), truncate(run.Error, maxStoredOutput))
	if err != nil {
		return fmt.Errorf("store: record task run: %w", err)
	}
	return nil
}

// RecentTaskRuns returns what the panel has done lately, newest first.
func (s *Store) RecentTaskRuns(ctx context.Context, serverID string, limit int) ([]TaskRun, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT name, ran_at, error FROM automation_runs
		   WHERE server_id = ? ORDER BY ran_at DESC, id DESC LIMIT ?`, serverID, limit)
	if err != nil {
		return nil, fmt.Errorf("store: list task runs: %w", err)
	}
	defer rows.Close()

	var out []TaskRun
	for rows.Next() {
		var r TaskRun
		var ranAt int64
		if err := rows.Scan(&r.Name, &ranAt, &r.Error); err != nil {
			return nil, fmt.Errorf("store: scan task run: %w", err)
		}
		r.RanAt = time.Unix(ranAt, 0).UTC()
		out = append(out, r)
	}
	return out, rows.Err()
}

// PruneTaskRuns keeps the newest rows per server.
func (s *Store) PruneTaskRuns(ctx context.Context, keep int) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM automation_runs WHERE id NOT IN (
		   SELECT id FROM automation_runs r2
		     WHERE r2.server_id = automation_runs.server_id
		     ORDER BY ran_at DESC, id DESC LIMIT ?)`, keep)
	if err != nil {
		return 0, fmt.Errorf("store: prune task runs: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// Task reads one task by name.
func (s *Store) Task(ctx context.Context, serverID, name string) (Task, error) {
	tasks, err := s.Tasks(ctx, serverID)
	if err != nil {
		return Task{}, err
	}
	for _, t := range tasks {
		if t.Name == name {
			return t, nil
		}
	}
	return Task{}, ErrNotFound
}

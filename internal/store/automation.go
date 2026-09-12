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
	// TriggerGametime fires at an hour of the game's day, every in-game day.
	// Dusk on a world with thirty-minute days comes round twice an hour.
	TriggerGametime TriggerKind = "gametime"
	// TriggerUptime fires once the server has been up long enough, which is
	// what a restart should really be keyed to rather than a wall clock.
	TriggerUptime TriggerKind = "uptime"
	// TriggerEmpty fires when the last player leaves.
	TriggerEmpty TriggerKind = "empty"
	// TriggerBloodmoonOver fires when a horde is survived.
	TriggerBloodmoonOver TriggerKind = "bloodmoonover"

	// TriggerJoin, TriggerLeave and TriggerDeath come off the event stream
	// rather than the clock.
	TriggerJoin  TriggerKind = "join"
	TriggerLeave TriggerKind = "leave"
	TriggerDeath TriggerKind = "death"
)

// EventTriggers are the ones set off by something happening rather than some
// time arriving. They never fire on a clock tick.
func (k TriggerKind) IsEvent() bool {
	switch k {
	case TriggerJoin, TriggerLeave, TriggerDeath:
		return true
	}
	return false
}

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

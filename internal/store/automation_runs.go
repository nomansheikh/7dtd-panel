package store

import (
	"context"
	"fmt"
	"time"
)

// The record of what the panel did on its own. Kept apart from the tasks
// themselves: one is the instruction, the other is the receipt, and only the
// receipt needs pruning.

// TaskRun is one record of a task going off.
type TaskRun struct {
	Name  string    `json:"name"`
	RanAt time.Time `json:"ranAt"`
	Error string    `json:"error,omitempty"`
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

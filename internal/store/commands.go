package store

import (
	"context"
	"fmt"
	"time"
)

// CommandRun is one console command an operator executed.
type CommandRun struct {
	ID        int64
	UserID    int64
	Command   string
	Succeeded bool
	Result    string
	Error     string
	CreatedAt time.Time
}

// maxStoredOutput caps what is retained per command. Some commands return very
// large output, and the history exists for recall rather than archival.
const maxStoredOutput = 8 * 1024

// AddCommandRun records an executed command and returns it with its id set.
func (s *Store) AddCommandRun(ctx context.Context, run CommandRun) (CommandRun, error) {
	if run.CreatedAt.IsZero() {
		run.CreatedAt = time.Now().UTC()
	}
	run.Result = truncate(run.Result, maxStoredOutput)
	run.Error = truncate(run.Error, maxStoredOutput)

	res, err := s.db.ExecContext(ctx, `
		INSERT INTO command_history (user_id, command, succeeded, result, error, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		run.UserID, run.Command, boolToInt(run.Succeeded), run.Result, run.Error,
		run.CreatedAt.Unix())
	if err != nil {
		return CommandRun{}, fmt.Errorf("store: record command: %w", err)
	}
	if run.ID, err = res.LastInsertId(); err != nil {
		return CommandRun{}, fmt.Errorf("store: command id: %w", err)
	}
	return run, nil
}

// RecentCommandRuns returns a user's most recent commands, newest first.
func (s *Store) RecentCommandRuns(ctx context.Context, userID int64, limit int) ([]CommandRun, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, command, succeeded, result, error, created_at
		FROM command_history
		WHERE user_id = ?
		ORDER BY id DESC
		LIMIT ?`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("store: read command history: %w", err)
	}
	defer rows.Close()

	var out []CommandRun
	for rows.Next() {
		var (
			run       CommandRun
			succeeded int
			created   int64
		)
		if err := rows.Scan(&run.ID, &run.UserID, &run.Command, &succeeded,
			&run.Result, &run.Error, &created); err != nil {
			return nil, fmt.Errorf("store: scan command history: %w", err)
		}
		run.Succeeded = succeeded != 0
		run.CreatedAt = time.Unix(created, 0).UTC()
		out = append(out, run)
	}
	return out, rows.Err()
}

// PruneCommandHistory keeps only the newest keep rows per user, so a long-lived
// panel does not accumulate history without bound.
func (s *Store) PruneCommandHistory(ctx context.Context, keep int) (int64, error) {
	if keep < 1 {
		keep = 1
	}
	res, err := s.db.ExecContext(ctx, `
		DELETE FROM command_history
		WHERE id NOT IN (
			SELECT id FROM (
				SELECT id, ROW_NUMBER() OVER (PARTITION BY user_id ORDER BY id DESC) AS rn
				FROM command_history
			) WHERE rn <= ?
		)`, keep)
	if err != nil {
		return 0, fmt.Errorf("store: prune command history: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, nil
	}
	return n, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n… truncated"
}

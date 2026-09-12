package store

import (
	"context"
	"fmt"
	"time"
)

// The bookkeeping that decides whether a task runs, and stops it running twice.
// Separate from saving a task, because a task is edited by a person and claimed
// by a poller, and only one of those has to be safe against itself.

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

/*
SetTaskKey records what the world looks like to a task without running it.

The triggers that fire on a change — the last player leaving, a horde ending —
need to know they have already seen the state they are in, or they would fire
again every thirty seconds. Recording the state without running is how a task
disarms itself, so the next entry into a firing state is a change rather than
more of the same.
*/
func (s *Store) SetTaskKey(ctx context.Context, serverID, name string, expect Task, newKey string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE automation_tasks SET last_key = ?
		   WHERE server_id = ? AND name = ? AND last_key = ?`,
		newKey, serverID, name, expect.LastKey)
	if err != nil {
		return fmt.Errorf("store: set task key: %w", err)
	}
	return nil
}

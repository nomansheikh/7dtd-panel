package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// User is a panel login. The panel has exactly one, seeded from the
// environment; the table exists so adding more later is a migration rather
// than a redesign.
type User struct {
	ID           int64
	Username     string
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// UserByUsername looks a user up by exact username.
func (s *Store) UserByUsername(ctx context.Context, username string) (User, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, username, password_hash, created_at, updated_at
		FROM users WHERE username = ?`, username)
	return scanUser(row)
}

// UserByID looks a user up by primary key.
func (s *Store) UserByID(ctx context.Context, id int64) (User, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, username, password_hash, created_at, updated_at
		FROM users WHERE id = ?`, id)
	return scanUser(row)
}

func scanUser(row *sql.Row) (User, error) {
	var u User
	var created, updated int64
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("store: scan user: %w", err)
	}
	u.CreatedAt = time.Unix(created, 0).UTC()
	u.UpdatedAt = time.Unix(updated, 0).UTC()
	return u, nil
}

// UpsertAdmin reconciles the single admin account with the supplied hash and
// reports whether anything changed.
//
// This runs on every boot, not only the first, so "set PANEL_ADMIN_PASSWORD,
// restart, you are back in" always holds. First-run-only seeding is a
// well-known way to lock people out of their own panel. The trade-off, which
// the README states, is that there is no password change in the UI: a restart
// would revert it.
func (s *Store) UpsertAdmin(ctx context.Context, username, passwordHash string) (User, bool, error) {
	if username == "" || passwordHash == "" {
		return User{}, false, errors.New("store: username and passwordHash are required")
	}
	now := time.Now().Unix()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, false, fmt.Errorf("store: begin upsert admin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var (
		id          int64
		currentHash string
	)
	err = tx.QueryRowContext(ctx,
		"SELECT id, password_hash FROM users WHERE username = ?", username).Scan(&id, &currentHash)

	switch {
	case errors.Is(err, sql.ErrNoRows):
		res, err := tx.ExecContext(ctx, `
			INSERT INTO users (username, password_hash, created_at, updated_at)
			VALUES (?, ?, ?, ?)`, username, passwordHash, now, now)
		if err != nil {
			return User{}, false, fmt.Errorf("store: insert admin: %w", err)
		}
		if id, err = res.LastInsertId(); err != nil {
			return User{}, false, fmt.Errorf("store: admin id: %w", err)
		}
	case err != nil:
		return User{}, false, fmt.Errorf("store: look up admin: %w", err)
	case currentHash == passwordHash:
		// Hash is byte-identical, so nothing to do. Note that a correct
		// password produces a different hash every time thanks to the salt, so
		// callers decide whether to rehash; see auth.VerifyPassword.
		if err := tx.Commit(); err != nil {
			return User{}, false, fmt.Errorf("store: commit upsert admin: %w", err)
		}
		u, err := s.UserByID(ctx, id)
		return u, false, err
	default:
		if _, err := tx.ExecContext(ctx,
			"UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ?",
			passwordHash, now, id); err != nil {
			return User{}, false, fmt.Errorf("store: update admin: %w", err)
		}
		// A changed password must not leave old sessions usable.
		if _, err := tx.ExecContext(ctx, "DELETE FROM sessions WHERE user_id = ?", id); err != nil {
			return User{}, false, fmt.Errorf("store: clear sessions: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return User{}, false, fmt.Errorf("store: commit upsert admin: %w", err)
	}
	u, err := s.UserByID(ctx, id)
	return u, true, err
}

package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Session is a logged-in browser. TokenHash is a SHA-256 of the cookie value;
// the token itself is never stored, so a leaked database cannot be replayed.
type Session struct {
	TokenHash string
	UserID    int64
	CreatedAt time.Time
	ExpiresAt time.Time
	UserAgent string
	IP        string
}

// CreateSession stores a new session.
func (s *Store) CreateSession(ctx context.Context, sess Session) error {
	if sess.TokenHash == "" {
		return errors.New("store: session token hash is required")
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO sessions (token_hash, user_id, created_at, expires_at, user_agent, ip)
		VALUES (?, ?, ?, ?, ?, ?)`,
		sess.TokenHash, sess.UserID, sess.CreatedAt.Unix(), sess.ExpiresAt.Unix(),
		sess.UserAgent, sess.IP)
	if err != nil {
		return fmt.Errorf("store: create session: %w", err)
	}
	return nil
}

// SessionUser returns the session and its user for a token hash.
//
// An expired session is reported as ErrNotFound and deleted, so expiry is
// enforced on read rather than relying on the cleanup sweep having run.
func (s *Store) SessionUser(ctx context.Context, tokenHash string, now time.Time) (Session, User, error) {
	var (
		sess             Session
		user             User
		created, expires int64
		userCreated      int64
		userUpdated      int64
	)
	err := s.db.QueryRowContext(ctx, `
		SELECT s.token_hash, s.user_id, s.created_at, s.expires_at, s.user_agent, s.ip,
		       u.id, u.username, u.password_hash, u.created_at, u.updated_at
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = ?`, tokenHash).Scan(
		&sess.TokenHash, &sess.UserID, &created, &expires, &sess.UserAgent, &sess.IP,
		&user.ID, &user.Username, &user.PasswordHash, &userCreated, &userUpdated)

	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, User{}, ErrNotFound
	}
	if err != nil {
		return Session{}, User{}, fmt.Errorf("store: look up session: %w", err)
	}

	sess.CreatedAt = time.Unix(created, 0).UTC()
	sess.ExpiresAt = time.Unix(expires, 0).UTC()
	user.CreatedAt = time.Unix(userCreated, 0).UTC()
	user.UpdatedAt = time.Unix(userUpdated, 0).UTC()

	if !now.Before(sess.ExpiresAt) {
		_ = s.DeleteSession(ctx, tokenHash)
		return Session{}, User{}, ErrNotFound
	}
	return sess, user, nil
}

// DeleteSession removes one session. Deleting an absent session is not an
// error, so logout is idempotent.
func (s *Store) DeleteSession(ctx context.Context, tokenHash string) error {
	if _, err := s.db.ExecContext(ctx,
		"DELETE FROM sessions WHERE token_hash = ?", tokenHash); err != nil {
		return fmt.Errorf("store: delete session: %w", err)
	}
	return nil
}

// DeleteExpiredSessions removes sessions that have lapsed and reports how many.
func (s *Store) DeleteExpiredSessions(ctx context.Context, now time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		"DELETE FROM sessions WHERE expires_at <= ?", now.Unix())
	if err != nil {
		return 0, fmt.Errorf("store: delete expired sessions: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, nil
	}
	return n, nil
}

// CountSessions reports how many sessions exist, for tests and diagnostics.
func (s *Store) CountSessions(ctx context.Context) (int, error) {
	var n int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sessions").Scan(&n); err != nil {
		return 0, fmt.Errorf("store: count sessions: %w", err)
	}
	return n, nil
}

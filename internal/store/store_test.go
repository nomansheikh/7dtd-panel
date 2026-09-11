package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	// A file rather than :memory: so WAL and the pragmas behave as in
	// production. t.TempDir cleans it up.
	path := filepath.Join(t.TempDir(), "panel.db")
	s, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestOpenCreatesParentDirectory(t *testing.T) {
	// The usual deployment mounts an empty volume, so the directory may be
	// several levels deep and absent.
	path := filepath.Join(t.TempDir(), "nested", "deeper", "panel.db")
	s, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	if _, err := s.DB().ExecContext(context.Background(), "SELECT 1"); err != nil {
		t.Errorf("database unusable: %v", err)
	}
}

func TestOpenRequiresPath(t *testing.T) {
	if _, err := Open(context.Background(), ""); err == nil {
		t.Error("Open succeeded with an empty path")
	}
}

func TestMigrationsAreAppliedOnceAndAreIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "panel.db")
	ctx := context.Background()

	s1, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	first, err := s1.AppliedMigrations(ctx)
	if err != nil {
		t.Fatalf("AppliedMigrations: %v", err)
	}
	if len(first) == 0 {
		t.Fatal("no migrations recorded")
	}
	_ = s1.Close()

	// Reopening must not reapply, which would fail on CREATE TABLE.
	s2, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s2.Close()

	second, err := s2.AppliedMigrations(ctx)
	if err != nil {
		t.Fatalf("AppliedMigrations: %v", err)
	}
	if len(second) != len(first) {
		t.Errorf("migrations went from %d to %d on reopen", len(first), len(second))
	}
}

func TestPragmasAreSet(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	tests := []struct {
		pragma string
		want   string
	}{
		{"journal_mode", "wal"},
		{"foreign_keys", "1"},
	}
	for _, tt := range tests {
		t.Run(tt.pragma, func(t *testing.T) {
			var got string
			if err := s.DB().QueryRowContext(ctx, "PRAGMA "+tt.pragma).Scan(&got); err != nil {
				t.Fatalf("read pragma: %v", err)
			}
			if got != tt.want {
				t.Errorf("%s = %q, want %q", tt.pragma, got, tt.want)
			}
		})
	}
}

func TestUpsertAdminCreatesThenUpdates(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	user, changed, err := s.UpsertAdmin(ctx, "admin", "hash-one")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !changed {
		t.Error("creating an admin should report a change")
	}
	if user.Username != "admin" || user.PasswordHash != "hash-one" {
		t.Errorf("unexpected user %+v", user)
	}

	t.Run("identical hash is a no-op", func(t *testing.T) {
		_, changed, err := s.UpsertAdmin(ctx, "admin", "hash-one")
		if err != nil {
			t.Fatalf("upsert: %v", err)
		}
		if changed {
			t.Error("an identical hash should not report a change")
		}
	})

	t.Run("new hash updates in place", func(t *testing.T) {
		updated, changed, err := s.UpsertAdmin(ctx, "admin", "hash-two")
		if err != nil {
			t.Fatalf("upsert: %v", err)
		}
		if !changed {
			t.Error("a new hash should report a change")
		}
		if updated.ID != user.ID {
			t.Errorf("id changed from %d to %d; the account should be updated, not replaced",
				user.ID, updated.ID)
		}
		if updated.PasswordHash != "hash-two" {
			t.Errorf("hash = %q, want hash-two", updated.PasswordHash)
		}
	})
}

func TestUpsertAdminRevokesSessionsOnPasswordChange(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	user, _, err := s.UpsertAdmin(ctx, "admin", "hash-one")
	if err != nil {
		t.Fatalf("UpsertAdmin: %v", err)
	}
	now := time.Now().UTC()
	if err := s.CreateSession(ctx, Session{
		TokenHash: "abc", UserID: user.ID,
		CreatedAt: now, ExpiresAt: now.Add(time.Hour),
	}); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	if n, _ := s.CountSessions(ctx); n != 1 {
		t.Fatalf("sessions = %d, want 1", n)
	}

	// Changing the password must not leave old cookies working.
	if _, _, err := s.UpsertAdmin(ctx, "admin", "hash-two"); err != nil {
		t.Fatalf("UpsertAdmin: %v", err)
	}
	if n, _ := s.CountSessions(ctx); n != 0 {
		t.Errorf("sessions = %d after password change, want 0", n)
	}
}

func TestUpsertAdminValidates(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	for _, tt := range []struct{ name, user, hash string }{
		{"no username", "", "h"},
		{"no hash", "admin", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, err := s.UpsertAdmin(ctx, tt.user, tt.hash); err == nil {
				t.Error("UpsertAdmin succeeded, want error")
			}
		})
	}
}

func TestUserByUsernameNotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.UserByUsername(context.Background(), "nobody")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}

func TestSessionLifecycle(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	user, _, err := s.UpsertAdmin(ctx, "admin", "hash")
	if err != nil {
		t.Fatalf("UpsertAdmin: %v", err)
	}
	now := time.Now().UTC().Truncate(time.Second)

	if err := s.CreateSession(ctx, Session{
		TokenHash: "hash-of-token",
		UserID:    user.ID,
		CreatedAt: now,
		ExpiresAt: now.Add(time.Hour),
		UserAgent: "test-agent",
		IP:        "10.0.0.1",
	}); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	t.Run("lookup returns session and user", func(t *testing.T) {
		sess, got, err := s.SessionUser(ctx, "hash-of-token", now)
		if err != nil {
			t.Fatalf("SessionUser: %v", err)
		}
		if got.ID != user.ID {
			t.Errorf("user id = %d, want %d", got.ID, user.ID)
		}
		if sess.UserAgent != "test-agent" || sess.IP != "10.0.0.1" {
			t.Errorf("unexpected session metadata %+v", sess)
		}
	})

	t.Run("unknown token is not found", func(t *testing.T) {
		if _, _, err := s.SessionUser(ctx, "nope", now); !errors.Is(err, ErrNotFound) {
			t.Errorf("error = %v, want ErrNotFound", err)
		}
	})

	t.Run("delete is idempotent", func(t *testing.T) {
		if err := s.DeleteSession(ctx, "hash-of-token"); err != nil {
			t.Fatalf("first delete: %v", err)
		}
		if err := s.DeleteSession(ctx, "hash-of-token"); err != nil {
			t.Errorf("second delete should succeed, got %v", err)
		}
	})
}

func TestExpiredSessionIsRejectedOnReadAndDeleted(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	user, _, err := s.UpsertAdmin(ctx, "admin", "hash")
	if err != nil {
		t.Fatalf("UpsertAdmin: %v", err)
	}
	now := time.Now().UTC()
	if err := s.CreateSession(ctx, Session{
		TokenHash: "stale",
		UserID:    user.ID,
		CreatedAt: now.Add(-2 * time.Hour),
		ExpiresAt: now.Add(-time.Hour),
	}); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// Expiry is enforced on read, not only by the sweeper, so a lapsed cookie
	// cannot be used in the window before cleanup runs.
	if _, _, err := s.SessionUser(ctx, "stale", now); !errors.Is(err, ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
	if n, _ := s.CountSessions(ctx); n != 0 {
		t.Errorf("sessions = %d; reading an expired session should delete it", n)
	}
}

func TestDeleteExpiredSessions(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	user, _, err := s.UpsertAdmin(ctx, "admin", "hash")
	if err != nil {
		t.Fatalf("UpsertAdmin: %v", err)
	}
	now := time.Now().UTC()
	for i, expires := range []time.Time{
		now.Add(-time.Hour),
		now.Add(-time.Minute),
		now.Add(time.Hour),
	} {
		if err := s.CreateSession(ctx, Session{
			TokenHash: string(rune('a' + i)),
			UserID:    user.ID,
			CreatedAt: now.Add(-2 * time.Hour),
			ExpiresAt: expires,
		}); err != nil {
			t.Fatalf("CreateSession: %v", err)
		}
	}

	n, err := s.DeleteExpiredSessions(ctx, now)
	if err != nil {
		t.Fatalf("DeleteExpiredSessions: %v", err)
	}
	if n != 2 {
		t.Errorf("deleted %d, want 2", n)
	}
	if left, _ := s.CountSessions(ctx); left != 1 {
		t.Errorf("remaining = %d, want 1", left)
	}
}

func TestSessionRequiresExistingUser(t *testing.T) {
	s := newTestStore(t)
	// foreign_keys is ON, so a session for a missing user must be rejected
	// rather than silently orphaned.
	err := s.CreateSession(context.Background(), Session{
		TokenHash: "x", UserID: 9999,
		CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour),
	})
	if err == nil {
		t.Error("CreateSession succeeded for a nonexistent user")
	}
}

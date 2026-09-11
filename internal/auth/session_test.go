package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/nomansheikh/7dtd-panel/internal/store"
)

func newSessions(t *testing.T, ttl time.Duration, trustProxy bool) (*Sessions, store.User) {
	t.Helper()
	db, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "panel.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	user, _, err := db.UpsertAdmin(context.Background(), "admin", "irrelevant-hash")
	if err != nil {
		t.Fatalf("UpsertAdmin: %v", err)
	}
	return NewSessions(db, ttl, trustProxy), user
}

func TestSessionCreateAndLookup(t *testing.T) {
	s, user := newSessions(t, time.Hour, false)
	ctx := context.Background()

	token, expires, err := s.Create(ctx, user.ID, "Mozilla/5.0", "10.0.0.7")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if token == "" {
		t.Fatal("Create returned an empty token")
	}
	if !expires.After(time.Now()) {
		t.Errorf("expiry %s is not in the future", expires)
	}

	got, err := s.Lookup(ctx, token)
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if got.ID != user.ID {
		t.Errorf("user id = %d, want %d", got.ID, user.ID)
	}
}

func TestSessionTokensAreUniqueAndOnlyHashesArePersisted(t *testing.T) {
	s, user := newSessions(t, time.Hour, false)
	ctx := context.Background()

	a, _, err := s.Create(ctx, user.ID, "", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	b, _, err := s.Create(ctx, user.ID, "", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if a == b {
		t.Fatal("two sessions got the same token")
	}

	// The raw token must not be recoverable from the database, so a stolen
	// copy cannot be replayed as a live session.
	var found int
	err = s.store.DB().QueryRowContext(ctx,
		"SELECT COUNT(*) FROM sessions WHERE token_hash = ?", a).Scan(&found)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if found != 0 {
		t.Error("the raw token is stored in the database")
	}

	err = s.store.DB().QueryRowContext(ctx,
		"SELECT COUNT(*) FROM sessions WHERE token_hash = ?", HashToken(a)).Scan(&found)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if found != 1 {
		t.Error("the token hash is not stored")
	}
}

func TestSessionLookupRejectsBadTokens(t *testing.T) {
	s, _ := newSessions(t, time.Hour, false)
	for _, tt := range []struct{ name, token string }{
		{"empty", ""},
		{"unknown", "not-a-real-token"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := s.Lookup(context.Background(), tt.token); !errors.Is(err, ErrInvalidSession) {
				t.Errorf("error = %v, want ErrInvalidSession", err)
			}
		})
	}
}

func TestExpiredSessionIsInvalid(t *testing.T) {
	s, user := newSessions(t, time.Hour, false)
	ctx := context.Background()

	// Issue the session in the past so it has already lapsed.
	past := time.Now().Add(-3 * time.Hour)
	s.now = func() time.Time { return past }
	token, _, err := s.Create(ctx, user.ID, "", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	s.now = time.Now
	if _, err := s.Lookup(ctx, token); !errors.Is(err, ErrInvalidSession) {
		t.Errorf("error = %v, want ErrInvalidSession", err)
	}
}

func TestRevokeIsIdempotent(t *testing.T) {
	s, user := newSessions(t, time.Hour, false)
	ctx := context.Background()

	token, _, err := s.Create(ctx, user.ID, "", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := s.Revoke(ctx, token); err != nil {
		t.Fatalf("first Revoke: %v", err)
	}
	if err := s.Revoke(ctx, token); err != nil {
		t.Errorf("second Revoke should succeed, got %v", err)
	}
	if err := s.Revoke(ctx, ""); err != nil {
		t.Errorf("revoking an empty token should succeed, got %v", err)
	}
	if _, err := s.Lookup(ctx, token); !errors.Is(err, ErrInvalidSession) {
		t.Error("a revoked session still resolves")
	}
}

func TestCleanupRemovesOnlyExpired(t *testing.T) {
	s, user := newSessions(t, time.Hour, false)
	ctx := context.Background()

	s.now = func() time.Time { return time.Now().Add(-3 * time.Hour) }
	if _, _, err := s.Create(ctx, user.ID, "", ""); err != nil {
		t.Fatalf("Create: %v", err)
	}
	s.now = time.Now
	live, _, err := s.Create(ctx, user.ID, "", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	n, err := s.Cleanup(ctx)
	if err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	if n != 1 {
		t.Errorf("cleaned %d, want 1", n)
	}
	if _, err := s.Lookup(ctx, live); err != nil {
		t.Errorf("cleanup removed a live session: %v", err)
	}
}

func TestLongUserAgentIsTruncated(t *testing.T) {
	s, user := newSessions(t, time.Hour, false)
	ctx := context.Background()

	huge := make([]byte, 4096)
	for i := range huge {
		huge[i] = 'a'
	}
	token, _, err := s.Create(ctx, user.ID, string(huge), "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	sess, _, err := s.store.SessionUser(ctx, HashToken(token), time.Now())
	if err != nil {
		t.Fatalf("SessionUser: %v", err)
	}
	if len(sess.UserAgent) > 512 {
		t.Errorf("user agent stored at %d bytes, want at most 512", len(sess.UserAgent))
	}
}

func TestCookieAttributes(t *testing.T) {
	s, _ := newSessions(t, time.Hour, false)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	s.SetCookie(rec, req, "the-token", time.Now().Add(time.Hour))

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("got %d cookies, want 1", len(cookies))
	}
	c := cookies[0]

	if c.Name != CookieName {
		t.Errorf("name = %q, want %q", c.Name, CookieName)
	}
	if !c.HttpOnly {
		t.Error("cookie must be HttpOnly so scripts cannot read it")
	}
	if c.SameSite != http.SameSiteLaxMode {
		t.Errorf("SameSite = %v, want Lax", c.SameSite)
	}
	if c.Path != "/" {
		t.Errorf("path = %q, want /", c.Path)
	}
	if c.Secure {
		t.Error("Secure must not be set on a plain HTTP request, or LAN users cannot log in")
	}
}

func TestSecureCookieDependsOnTLSAndTrustProxy(t *testing.T) {
	tests := []struct {
		name       string
		trustProxy bool
		tls        bool
		forwarded  string
		wantSecure bool
	}{
		{name: "plain http", wantSecure: false},
		{name: "direct tls", tls: true, wantSecure: true},
		{
			// Believing this header without opting in would let any client set
			// Secure on its own cookie.
			name:       "forwarded proto ignored when proxy is untrusted",
			forwarded:  "https",
			wantSecure: false,
		},
		{
			name:       "forwarded proto honoured when proxy is trusted",
			trustProxy: true,
			forwarded:  "https",
			wantSecure: true,
		},
		{
			name:       "forwarded http stays insecure",
			trustProxy: true,
			forwarded:  "http",
			wantSecure: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, _ := newSessions(t, time.Hour, tt.trustProxy)
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.tls {
				req.TLS = &tlsState
			}
			if tt.forwarded != "" {
				req.Header.Set("X-Forwarded-Proto", tt.forwarded)
			}
			if got := s.IsSecure(req); got != tt.wantSecure {
				t.Errorf("IsSecure = %v, want %v", got, tt.wantSecure)
			}
		})
	}
}

func TestClearCookieExpiresIt(t *testing.T) {
	s, _ := newSessions(t, time.Hour, false)
	rec := httptest.NewRecorder()
	s.ClearCookie(rec, httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil))

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("got %d cookies, want 1", len(cookies))
	}
	if cookies[0].MaxAge >= 0 {
		t.Errorf("MaxAge = %d, want negative so the browser drops it", cookies[0].MaxAge)
	}
	if cookies[0].Value != "" {
		t.Errorf("value = %q, want empty", cookies[0].Value)
	}
}

func TestTokenFromRequest(t *testing.T) {
	t.Run("present", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: CookieName, Value: "abc"})
		if got := TokenFromRequest(req); got != "abc" {
			t.Errorf("got %q, want abc", got)
		}
	})
	t.Run("absent", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if got := TokenFromRequest(req); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
}

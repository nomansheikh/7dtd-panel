package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/nomansheikh/7dtd-panel/internal/store"
)

// CookieName is the session cookie. The panel is often reverse-proxied
// alongside other tools, so the name is specific enough not to collide.
const CookieName = "sdtd_panel_session"

// tokenBytes is the session token's entropy. 32 bytes is well beyond
// guessability and keeps the cookie short.
const tokenBytes = 32

// ErrInvalidSession means the token is unknown, expired or malformed.
var ErrInvalidSession = errors.New("auth: invalid session")

// Sessions issues and validates session cookies.
type Sessions struct {
	store *store.Store
	ttl   time.Duration
	// secure forces the Secure cookie attribute. It is derived per request
	// rather than fixed, because the panel may be served over plain HTTP on a
	// LAN or behind a TLS proxy.
	trustProxy bool
	now        func() time.Time
}

// NewSessions builds a session manager.
func NewSessions(s *store.Store, ttl time.Duration, trustProxy bool) *Sessions {
	return &Sessions{store: s, ttl: ttl, trustProxy: trustProxy, now: time.Now}
}

// Create issues a session for a user and returns the raw token to put in a
// cookie. Only the hash is persisted.
func (s *Sessions) Create(ctx context.Context, userID int64, userAgent, ip string) (string, time.Time, error) {
	raw := make([]byte, tokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", time.Time{}, fmt.Errorf("auth: generate session token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)

	now := s.now().UTC()
	expires := now.Add(s.ttl)

	// A hostile or broken client could send a megabyte of User-Agent.
	if len(userAgent) > 512 {
		userAgent = userAgent[:512]
	}

	err := s.store.CreateSession(ctx, store.Session{
		TokenHash: HashToken(token),
		UserID:    userID,
		CreatedAt: now,
		ExpiresAt: expires,
		UserAgent: userAgent,
		IP:        ip,
	})
	if err != nil {
		return "", time.Time{}, err
	}
	return token, expires, nil
}

// Lookup resolves a raw token to its user.
func (s *Sessions) Lookup(ctx context.Context, token string) (store.User, error) {
	if token == "" {
		return store.User{}, ErrInvalidSession
	}
	_, user, err := s.store.SessionUser(ctx, HashToken(token), s.now().UTC())
	if errors.Is(err, store.ErrNotFound) {
		return store.User{}, ErrInvalidSession
	}
	if err != nil {
		return store.User{}, err
	}
	return user, nil
}

// Revoke deletes a session. Revoking an unknown token is not an error, so
// logout is idempotent.
func (s *Sessions) Revoke(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.store.DeleteSession(ctx, HashToken(token))
}

// Cleanup removes expired sessions and reports how many went.
func (s *Sessions) Cleanup(ctx context.Context) (int64, error) {
	return s.store.DeleteExpiredSessions(ctx, s.now().UTC())
}

// HashToken returns the hex SHA-256 of a session token. Exported so the store
// and tests agree on the representation.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// SetCookie writes the session cookie.
//
// HttpOnly keeps it away from JavaScript, SameSite=Lax blocks cross-site form
// posts while leaving ordinary navigation working, and Secure is set whenever
// the request arrived over TLS.
func (s *Sessions) SetCookie(w http.ResponseWriter, r *http.Request, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		MaxAge:   int(time.Until(expires).Seconds()),
		HttpOnly: true,
		Secure:   s.IsSecure(r),
		SameSite: http.SameSiteLaxMode,
	})
}

// ClearCookie expires the session cookie.
func (s *Sessions) ClearCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.IsSecure(r),
		SameSite: http.SameSiteLaxMode,
	})
}

// IsSecure reports whether the request reached us over TLS.
//
// X-Forwarded-Proto is only believed when PANEL_TRUST_PROXY is on. Trusting it
// unconditionally would let any client set Secure on its own cookie, and
// ignoring it always would break every TLS-terminating reverse proxy.
func (s *Sessions) IsSecure(r *http.Request) bool {
	if r == nil {
		return false
	}
	if r.TLS != nil {
		return true
	}
	if s.trustProxy && r.Header.Get("X-Forwarded-Proto") == "https" {
		return true
	}
	return false
}

// TokenFromRequest reads the session token out of the request cookie.
func TokenFromRequest(r *http.Request) string {
	c, err := r.Cookie(CookieName)
	if err != nil {
		return ""
	}
	return c.Value
}

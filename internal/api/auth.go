package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/nomansheikh/7dtd-panel/internal/auth"
	"github.com/nomansheikh/7dtd-panel/internal/httpx"
	"github.com/nomansheikh/7dtd-panel/internal/store"
)

type userCtxKey struct{}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type userResponse struct {
	Username string `json:"username"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	ip := httpx.ClientIP(r, s.cfg.Panel.TrustProxy)
	if !s.loginLimiter.Allow(ip, s.now()) {
		httpx.WriteError(w, http.StatusTooManyRequests,
			"too many login attempts; wait a minute and try again", "RATE_LIMITED")
		return
	}

	var req loginRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error(), "INVALID_BODY")
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || req.Password == "" {
		httpx.WriteError(w, http.StatusBadRequest,
			"username and password are required", "MISSING_CREDENTIALS")
		return
	}

	user, err := s.store.UserByUsername(r.Context(), req.Username)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		s.log.Error("login lookup failed", "error", err)
		httpx.WriteError(w, http.StatusInternalServerError,
			"could not read the user database", "STORE_ERROR")
		return
	}

	// An unknown username still pays for a hash verification, so response time
	// does not reveal which accounts exist.
	storedHash := user.PasswordHash
	if errors.Is(err, store.ErrNotFound) {
		storedHash = dummyHash
	}

	ok, verifyErr := auth.VerifyPassword(storedHash, req.Password)
	if verifyErr != nil && !errors.Is(err, store.ErrNotFound) {
		s.log.Error("stored password hash is unusable",
			"username", req.Username, "error", verifyErr)
		httpx.WriteError(w, http.StatusInternalServerError,
			"the stored password hash is corrupt; restart with PANEL_ADMIN_PASSWORD set to reset it",
			"BAD_HASH")
		return
	}
	if !ok || errors.Is(err, store.ErrNotFound) {
		s.log.Warn("failed login", "username", req.Username, "remoteIp", ip)
		httpx.WriteError(w, http.StatusUnauthorized,
			"incorrect username or password", "INVALID_CREDENTIALS")
		return
	}

	token, expires, err := s.sessions.Create(r.Context(), user.ID, r.UserAgent(), ip)
	if err != nil {
		s.log.Error("session creation failed", "error", err)
		httpx.WriteError(w, http.StatusInternalServerError,
			"could not create a session", "STORE_ERROR")
		return
	}

	s.loginLimiter.Reset(ip)
	s.sessions.SetCookie(w, r, token, expires)
	s.log.Info("login succeeded", "username", user.Username, "remoteIp", ip)
	httpx.WriteJSON(w, http.StatusOK, userResponse{Username: user.Username})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	token := auth.TokenFromRequest(r)
	if err := s.sessions.Revoke(r.Context(), token); err != nil {
		s.log.Warn("session revoke failed", "error", err)
	}
	s.sessions.ClearCookie(w, r)
	// Logout is idempotent: logging out when already logged out succeeds.
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFrom(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "not authenticated", "UNAUTHENTICATED")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, userResponse{Username: user.Username})
}

// requireAuth rejects requests without a valid session.
//
// It also enforces same-origin on mutating methods. With SameSite=Lax cookies
// and a same-origin SPA that is sufficient CSRF protection without a token
// exchange, and it fails closed when a browser sends neither header.
func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isMutating(r.Method) && !s.sameOrigin(r) {
			httpx.WriteError(w, http.StatusForbidden,
				"cross-origin request rejected", "CROSS_ORIGIN")
			return
		}

		user, err := s.sessions.Lookup(r.Context(), auth.TokenFromRequest(r))
		if err != nil {
			if !errors.Is(err, auth.ErrInvalidSession) {
				s.log.Error("session lookup failed", "error", err)
			}
			httpx.WriteError(w, http.StatusUnauthorized,
				"not authenticated", "UNAUTHENTICATED")
			return
		}
		next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), user)))
	})
}

func isMutating(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	default:
		return true
	}
}

// sameOrigin reports whether a mutating request came from the panel's own UI.
func (s *Server) sameOrigin(r *http.Request) bool {
	// Fetch metadata is the most reliable signal and is sent by every current
	// browser.
	switch r.Header.Get("Sec-Fetch-Site") {
	case "same-origin", "none":
		return true
	case "cross-site", "same-site":
		return false
	}

	origin := r.Header.Get("Origin")
	if origin == "" {
		// Non-browser clients such as curl send neither header. They cannot be
		// the target of a CSRF attack, since that requires a browser with the
		// cookie.
		return true
	}
	return strings.EqualFold(origin, s.expectedOrigin(r))
}

func (s *Server) expectedOrigin(r *http.Request) string {
	scheme := "http"
	if s.sessions.IsSecure(r) {
		scheme = "https"
	}
	host := r.Host
	if s.cfg.Panel.TrustProxy {
		if fwd := r.Header.Get("X-Forwarded-Host"); fwd != "" {
			host = fwd
		}
	}
	return scheme + "://" + host
}

// WithUser stores the authenticated user on the context.
func WithUser(ctx context.Context, u store.User) context.Context {
	return context.WithValue(ctx, userCtxKey{}, u)
}

// UserFrom retrieves the authenticated user.
func UserFrom(ctx context.Context) (store.User, bool) {
	u, ok := ctx.Value(userCtxKey{}).(store.User)
	return u, ok
}

// dummyHash is a real argon2id hash of an unguessable value. Verifying against
// it for unknown usernames keeps the failure path the same cost as the success
// path, so timing does not leak which accounts exist.
const dummyHash = "$argon2id$v=19$m=65536,t=3,p=4$" +
	"c2FsdHNhbHRzYWx0c2Fs$" +
	"YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTY"

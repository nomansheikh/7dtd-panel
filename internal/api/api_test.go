package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nomansheikh/7dtd-panel/internal/auth"
	"github.com/nomansheikh/7dtd-panel/internal/config"
	"github.com/nomansheikh/7dtd-panel/internal/state"
	"github.com/nomansheikh/7dtd-panel/internal/store"
)

const testPassword = "a-good-test-password"

// argon2 is deliberately expensive, and the harness is built per test. Hashing
// once keeps the suite from spending most of its time re-deriving the same
// value, which under -race dominated the package's runtime.
var testPasswordHash = sync.OnceValue(func() string {
	h, err := auth.HashPassword(testPassword)
	if err != nil {
		panic("hash test password: " + err.Error())
	}
	return h
})

type fakeState struct{ snap state.Snapshot }

func (f fakeState) Snapshot() state.Snapshot { return f.snap }

type harness struct {
	server  *Server
	handler http.Handler
	store   *store.Store
}

func newHarness(t *testing.T, snap state.Snapshot) *harness {
	t.Helper()

	db, err := store.Open(t.Context(), filepath.Join(t.TempDir(), "panel.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if _, _, err := db.UpsertAdmin(t.Context(), "admin", testPasswordHash()); err != nil {
		t.Fatalf("UpsertAdmin: %v", err)
	}

	cfg := config.Config{}
	cfg.Panel.AdminUsername = "admin"
	cfg.Panel.SessionTTL = time.Hour

	s := NewServer(Deps{
		Config:   cfg,
		Store:    db,
		Sessions: auth.NewSessions(db, time.Hour, false),
		State:    fakeState{snap: snap},
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		Version:  "test",
	})
	return &harness{server: s, handler: s.Routes(), store: db}
}

// login performs a real login and returns the session cookie.
func (h *harness) login(t *testing.T, username, password string) *http.Cookie {
	t.Helper()
	rec := h.do(t, h.request(t, http.MethodPost, "/api/auth/login",
		`{"username":"`+username+`","password":"`+password+`"}`, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("login returned %d: %s", rec.Code, rec.Body.String())
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.CookieName {
			return c
		}
	}
	t.Fatal("login did not set a session cookie")
	return nil
}

func (h *harness) request(t *testing.T, method, path, body string, cookie *http.Cookie) *http.Request {
	t.Helper()
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		r.AddCookie(cookie)
	}
	// Browsers send this; the same-origin check relies on it.
	r.Header.Set("Sec-Fetch-Site", "same-origin")
	return r
}

func (h *harness) do(t *testing.T, r *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, r)
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode %s: %v", rec.Body.String(), err)
	}
	return v
}

func TestHealthIsPublicAndSurvivesGameServerBeingDown(t *testing.T) {
	// The game server is a different machine. If its being down made this
	// endpoint fail, the orchestrator would restart the panel for no reason.
	h := newHarness(t, state.Snapshot{
		Status:    state.StatusOffline,
		LastError: "connection refused",
	})

	rec := h.do(t, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 even with the game server offline", rec.Code)
	}

	body := decode[map[string]any](t, rec)
	if body["status"] != "ok" {
		t.Errorf("panel status = %v, want ok", body["status"])
	}
	game, _ := body["game"].(map[string]any)
	if game["status"] != "offline" {
		t.Errorf("game status = %v, want offline", game["status"])
	}
	if game["error"] != "connection refused" {
		t.Errorf("game error = %v, want the real reason", game["error"])
	}
}

func TestHealthHidesErrorWhileMerelyDegraded(t *testing.T) {
	h := newHarness(t, state.Snapshot{Status: state.StatusDegraded, LastError: "timeout"})
	rec := h.do(t, httptest.NewRequest(http.MethodGet, "/api/health", nil))

	game, _ := decode[map[string]any](t, rec)["game"].(map[string]any)
	if _, present := game["error"]; present {
		t.Error("a single blip should not surface an error in health")
	}
}

func TestLoginSucceedsAndSetsASecureCookie(t *testing.T) {
	h := newHarness(t, state.Snapshot{})
	rec := h.do(t, h.request(t, http.MethodPost, "/api/auth/login",
		`{"username":"admin","password":"`+testPassword+`"}`, nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	if got := decode[map[string]any](t, rec)["username"]; got != "admin" {
		t.Errorf("username = %v, want admin", got)
	}

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("got %d cookies, want 1", len(cookies))
	}
	c := cookies[0]
	if !c.HttpOnly {
		t.Error("session cookie must be HttpOnly")
	}
	if c.SameSite != http.SameSiteLaxMode {
		t.Errorf("SameSite = %v, want Lax", c.SameSite)
	}
	// The response must never echo the password back.
	if strings.Contains(rec.Body.String(), testPassword) {
		t.Error("the login response contains the password")
	}
}

func TestLoginRejections(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		wantCode int
		wantErr  string
	}{
		{
			name:     "wrong password",
			body:     `{"username":"admin","password":"wrong"}`,
			wantCode: http.StatusUnauthorized,
			wantErr:  "INVALID_CREDENTIALS",
		},
		{
			// Same response as a wrong password, so the error does not reveal
			// which usernames exist.
			name:     "unknown user",
			body:     `{"username":"nobody","password":"` + testPassword + `"}`,
			wantCode: http.StatusUnauthorized,
			wantErr:  "INVALID_CREDENTIALS",
		},
		{
			name:     "empty username",
			body:     `{"username":"","password":"x"}`,
			wantCode: http.StatusBadRequest,
			wantErr:  "MISSING_CREDENTIALS",
		},
		{
			name:     "empty password",
			body:     `{"username":"admin","password":""}`,
			wantCode: http.StatusBadRequest,
			wantErr:  "MISSING_CREDENTIALS",
		},
		{
			name:     "malformed json",
			body:     `{"username":`,
			wantCode: http.StatusBadRequest,
			wantErr:  "INVALID_BODY",
		},
		{
			name:     "unknown field is rejected",
			body:     `{"username":"admin","password":"x","admin":true}`,
			wantCode: http.StatusBadRequest,
			wantErr:  "INVALID_BODY",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t, state.Snapshot{})
			rec := h.do(t, h.request(t, http.MethodPost, "/api/auth/login", tt.body, nil))

			if rec.Code != tt.wantCode {
				t.Errorf("status = %d, want %d (body %s)", rec.Code, tt.wantCode, rec.Body.String())
			}
			if got := decode[map[string]any](t, rec)["code"]; got != tt.wantErr {
				t.Errorf("code = %v, want %v", got, tt.wantErr)
			}
			if len(rec.Result().Cookies()) != 0 {
				t.Error("a failed login must not set a cookie")
			}
		})
	}
}

func TestLoginIsRateLimited(t *testing.T) {
	h := newHarness(t, state.Snapshot{})

	var limited bool
	for i := 0; i < 15; i++ {
		rec := h.do(t, h.request(t, http.MethodPost, "/api/auth/login",
			`{"username":"admin","password":"wrong"}`, nil))
		if rec.Code == http.StatusTooManyRequests {
			limited = true
			break
		}
	}
	if !limited {
		t.Error("repeated failed logins were never rate limited")
	}
}

func TestSuccessfulLoginClearsTheRateLimit(t *testing.T) {
	h := newHarness(t, state.Snapshot{})

	// A legitimate operator who fumbles a few times should not stay throttled.
	for i := 0; i < 5; i++ {
		h.do(t, h.request(t, http.MethodPost, "/api/auth/login",
			`{"username":"admin","password":"wrong"}`, nil))
	}
	h.login(t, "admin", testPassword)

	for i := 0; i < 8; i++ {
		rec := h.do(t, h.request(t, http.MethodPost, "/api/auth/login",
			`{"username":"admin","password":"wrong"}`, nil))
		if rec.Code == http.StatusTooManyRequests {
			t.Fatalf("throttled again after %d attempts; the counter was not reset", i+1)
		}
	}
}

func TestAuthenticatedEndpointsRequireASession(t *testing.T) {
	h := newHarness(t, state.Snapshot{})
	for _, path := range []string{"/api/auth/me", "/api/dashboard"} {
		t.Run(path, func(t *testing.T) {
			rec := h.do(t, h.request(t, http.MethodGet, path, "", nil))
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401", rec.Code)
			}
			if got := decode[map[string]any](t, rec)["code"]; got != "UNAUTHENTICATED" {
				t.Errorf("code = %v, want UNAUTHENTICATED", got)
			}
		})
	}
}

func TestSessionCookieGrantsAccess(t *testing.T) {
	h := newHarness(t, state.Snapshot{Status: state.StatusOnline})
	cookie := h.login(t, "admin", testPassword)

	rec := h.do(t, h.request(t, http.MethodGet, "/api/auth/me", "", cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	if got := decode[map[string]any](t, rec)["username"]; got != "admin" {
		t.Errorf("username = %v, want admin", got)
	}
}

func TestGarbageCookieIsRejected(t *testing.T) {
	h := newHarness(t, state.Snapshot{})
	bad := &http.Cookie{Name: auth.CookieName, Value: "not-a-real-token"}

	rec := h.do(t, h.request(t, http.MethodGet, "/api/auth/me", "", bad))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestLogoutInvalidatesTheSession(t *testing.T) {
	h := newHarness(t, state.Snapshot{})
	cookie := h.login(t, "admin", testPassword)

	rec := h.do(t, h.request(t, http.MethodPost, "/api/auth/logout", "", cookie))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}

	// The same cookie must no longer work.
	after := h.do(t, h.request(t, http.MethodGet, "/api/auth/me", "", cookie))
	if after.Code != http.StatusUnauthorized {
		t.Errorf("status = %d after logout, want 401", after.Code)
	}
}

func TestLogoutWithoutASessionSucceeds(t *testing.T) {
	h := newHarness(t, state.Snapshot{})
	rec := h.do(t, h.request(t, http.MethodPost, "/api/auth/logout", "", nil))
	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204; logout should be idempotent", rec.Code)
	}
}

// TestRequireAuthSameOriginGuard exercises the CSRF guard directly, because
// every protected route is currently a GET and the guard only applies to
// mutating methods.
func TestRequireAuthSameOriginGuard(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		fetchSite  string
		origin     string
		wantStatus int
	}{
		{"same-origin post is allowed", http.MethodPost, "same-origin", "", http.StatusOK},
		{"direct navigation post is allowed", http.MethodPost, "none", "", http.StatusOK},
		{"cross-site post is rejected", http.MethodPost, "cross-site", "", http.StatusForbidden},
		{"same-site post is rejected", http.MethodPost, "same-site", "", http.StatusForbidden},
		{
			// curl and other non-browser clients send neither header. They
			// cannot be CSRF vectors, since that needs a browser holding the
			// cookie.
			name:       "post with no metadata is allowed",
			method:     http.MethodPost,
			wantStatus: http.StatusOK,
		},
		{
			name:       "foreign Origin is rejected",
			method:     http.MethodPost,
			origin:     "https://evil.example",
			wantStatus: http.StatusForbidden,
		},
		{"cross-site GET is allowed", http.MethodGet, "cross-site", "", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t, state.Snapshot{})
			cookie := h.login(t, "admin", testPassword)

			protected := h.server.requireAuth(http.HandlerFunc(
				func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))

			r := httptest.NewRequest(tt.method, "/api/whatever", nil)
			r.AddCookie(cookie)
			if tt.fetchSite != "" {
				r.Header.Set("Sec-Fetch-Site", tt.fetchSite)
			}
			if tt.origin != "" {
				r.Header.Set("Origin", tt.origin)
			}

			rec := httptest.NewRecorder()
			protected.ServeHTTP(rec, r)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d (body %s)", rec.Code, tt.wantStatus, rec.Body.String())
			}
		})
	}
}

func TestDashboardReportsStalenessInsteadOfBlanking(t *testing.T) {
	now := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	// Degraded: the figures are cached from 45s ago but must still be served.
	h := newHarness(t, state.Snapshot{
		Status:     state.StatusDegraded,
		StatsAt:    now.Add(-45 * time.Second),
		LastError:  "i/o timeout",
		Version:    "V.3.20.10",
		World:      "Navezgane",
		MaxPlayers: 8,
	})
	h.server.now = func() time.Time { return now }

	cookie := h.login(t, "admin", testPassword)
	rec := h.do(t, h.request(t, http.MethodGet, "/api/dashboard", "", cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}

	body := decode[map[string]any](t, rec)
	if body["status"] != "degraded" {
		t.Errorf("status = %v, want degraded", body["status"])
	}
	if body["stale"] != true {
		t.Error("stale should be true while degraded")
	}
	if age, _ := body["ageSeconds"].(float64); age != 45 {
		t.Errorf("ageSeconds = %v, want 45", body["ageSeconds"])
	}
	if body["lastError"] != "i/o timeout" {
		t.Errorf("lastError = %v, want the real reason", body["lastError"])
	}
	// Cached values must still be present rather than zeroed.
	server, _ := body["server"].(map[string]any)
	if server["version"] != "V.3.20.10" {
		t.Errorf("version = %v; cached data should survive a blip", server["version"])
	}
}

func TestDashboardOmitsUptimeAndMoonUntilSampled(t *testing.T) {
	h := newHarness(t, state.Snapshot{Status: state.StatusOnline, StatsAt: time.Now()})
	cookie := h.login(t, "admin", testPassword)

	body := decode[map[string]any](t, h.do(t,
		h.request(t, http.MethodGet, "/api/dashboard", "", cookie)))

	// Reporting zero would claim the server just started and that a blood moon
	// is due on day zero. Null is honest.
	if body["uptime"] != nil {
		t.Errorf("uptime = %v, want null before the first sample", body["uptime"])
	}
	if body["bloodMoon"] != nil {
		t.Errorf("bloodMoon = %v, want null before the first sample", body["bloodMoon"])
	}
}

func TestUnknownAPIRouteIs404(t *testing.T) {
	h := newHarness(t, state.Snapshot{})
	rec := h.do(t, h.request(t, http.MethodGet, "/api/nope", "", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestWrongMethodIsRejected(t *testing.T) {
	h := newHarness(t, state.Snapshot{})
	// Go 1.22 ServeMux returns 405 when a pattern matches a different method.
	rec := h.do(t, h.request(t, http.MethodGet, "/api/auth/login", "", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rec.Code)
	}
}

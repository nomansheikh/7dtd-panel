package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/nomansheikh/7dtd-panel/internal/state"
)

func TestServersListing(t *testing.T) {
	h := newHarness(t, state.Snapshot{
		Status:         state.StatusOnline,
		Version:        "V.3.20.10",
		World:          "Navezgane",
		MaxPlayers:     8,
		ConnectAddress: "203.0.113.10:26900",
	})
	cookie := h.login(t, "admin", testPassword)

	rec := h.do(t, h.request(t, http.MethodGet, "/api/servers", "", cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Servers []serverSummary `json:"servers"`
		Default string          `json:"default"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Servers) != 1 {
		t.Fatalf("got %d servers, want 1", len(body.Servers))
	}
	got := body.Servers[0]
	if got.ID != testServerID || got.Name != "Test server" {
		t.Errorf("summary = %+v", got)
	}
	// The switcher shows reachability without polling each dashboard.
	if got.Status != "online" {
		t.Errorf("status = %q, want online", got.Status)
	}
	if got.Connect != "203.0.113.10:26900" {
		t.Errorf("connect = %q", got.Connect)
	}
	if body.Default != testServerID {
		t.Errorf("default = %q, want %s", body.Default, testServerID)
	}
}

func TestUnknownServerIsRejected(t *testing.T) {
	h := newHarness(t, state.Snapshot{})
	cookie := h.login(t, "admin", testPassword)

	// Requesting a server that is not configured must be a clear 404 rather
	// than silently falling back to the default, which would show an operator
	// a different world than the URL names.
	req := h.request(t, http.MethodGet, "/api/auth/me", "", cookie)
	req.URL.Path = "/api/servers/nosuchserver/dashboard"

	rec := h.do(t, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	body := decode[map[string]any](t, rec)
	if body["code"] != "UNKNOWN_SERVER" {
		t.Errorf("code = %v, want UNKNOWN_SERVER", body["code"])
	}
	if got, _ := body["error"].(string); got == "" {
		t.Error("the error should name the server that was not found")
	}
}

func TestServerScopedEndpointsRequireAuth(t *testing.T) {
	h := newHarness(t, state.Snapshot{})
	rec := h.do(t, h.request(t, http.MethodGet, "/api/servers", "", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestDashboardCarriesServerIdentityAndJoinDetails(t *testing.T) {
	h := newHarness(t, state.Snapshot{
		Status:         state.StatusOnline,
		ServerName:     "Nomans 7DTD",
		Description:    "A 7 Days to Die server",
		Region:         "NorthAmericaEast",
		ConnectAddress: "203.0.113.10:26900",
	})
	cookie := h.login(t, "admin", testPassword)

	rec := h.do(t, h.request(t, http.MethodGet, "/api/dashboard", "", cookie))
	body := decode[map[string]any](t, rec)
	server, _ := body["server"].(map[string]any)

	if server["id"] != testServerID {
		t.Errorf("id = %v", server["id"])
	}
	// The name the game server reports beats the panel's configured label.
	if server["name"] != "Nomans 7DTD" {
		t.Errorf("name = %v, want the server's own name", server["name"])
	}
	if server["connect"] != "203.0.113.10:26900" {
		t.Errorf("connect = %v", server["connect"])
	}
	// panelUrl helps diagnose a misconfigured deployment and must never carry
	// credentials.
	if server["panelUrl"] != "http://game.test:8080" {
		t.Errorf("panelUrl = %v", server["panelUrl"])
	}
}

func TestDashboardFallsBackToTheConfiguredName(t *testing.T) {
	// Before the first serverinfo poll the game server has told us nothing, so
	// the label from configuration is all there is.
	h := newHarness(t, state.Snapshot{Status: state.StatusUnknown})
	cookie := h.login(t, "admin", testPassword)

	rec := h.do(t, h.request(t, http.MethodGet, "/api/dashboard", "", cookie))
	server, _ := decode[map[string]any](t, rec)["server"].(map[string]any)
	if server["name"] != "Test server" {
		t.Errorf("name = %v, want the configured name", server["name"])
	}
}

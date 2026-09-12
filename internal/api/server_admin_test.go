package api

import (
	"net/http"
	"testing"

	"github.com/nomansheikh/7dtd-panel/internal/servers"
	"github.com/nomansheikh/7dtd-panel/internal/state"
	"github.com/nomansheikh/7dtd-panel/internal/store"
)

// emptyRegistry is what a panel with nothing configured has.
func emptyRegistry() *servers.Registry { return servers.NewRegistry() }

func gameServerFixture() store.GameServer {
	return store.GameServer{
		ID: "extra", Name: "Extra", Host: "10.0.0.9", Port: 8080, Scheme: "http",
		TokenName: "panel", TokenSecret: "super-secret-value",
	}
}

/*
Adding a server from the panel.

The case that matters most is the one a new install hits: no servers at all.
The panel used to refuse to start; now it boots, says so, and offers a way in.
*/
func TestAFreshPanelReportsNoServersRatherThanFailing(t *testing.T) {
	h := newHarness(t, state.Snapshot{})
	// A registry with nothing in it, which is what a first boot looks like.
	h.server.registry = emptyRegistry()
	cookie := h.login(t, "admin", testPassword)

	rec := h.do(t, h.request(t, http.MethodGet, "/api/servers", "", cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	body := decode[struct {
		Servers []map[string]any `json:"servers"`
		Default string           `json:"default"`
	}](t, rec)

	if len(body.Servers) != 0 {
		t.Errorf("servers = %+v, want none", body.Servers)
	}
	// An empty default is what tells the UI to offer the wizard.
	if body.Default != "" {
		t.Errorf("default = %q, want empty", body.Default)
	}
}

func TestAddingAServerRejectsWhatCannotWork(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"no id", `{"host":"10.0.0.5","tokenName":"panel","tokenSecret":"s"}`},
		{"an id with spaces", `{"id":"my server","host":"10.0.0.5","tokenName":"p","tokenSecret":"s"}`},
		{"no host", `{"id":"main","tokenName":"panel","tokenSecret":"s"}`},
		{"no token name", `{"id":"main","host":"10.0.0.5","tokenSecret":"s"}`},
		{"no token secret", `{"id":"main","host":"10.0.0.5","tokenName":"panel"}`},
		{"a silly port", `{"id":"main","host":"10.0.0.5","port":99999,"tokenName":"p","tokenSecret":"s"}`},
		{"a scheme that is not http", `{"id":"main","host":"h","scheme":"ftp","tokenName":"p","tokenSecret":"s"}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, state.Snapshot{})
			cookie := h.login(t, "admin", testPassword)
			rec := h.do(t, h.request(t, http.MethodPost, "/api/servers", tc.body, cookie))
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

// The id is in every URL, so two servers cannot share one.
func TestAServerIdCannotBeTakenTwice(t *testing.T) {
	h := newHarness(t, state.Snapshot{})
	cookie := h.login(t, "admin", testPassword)

	// testServerID is already in the registry from the harness.
	rec := h.do(t, h.request(t, http.MethodPost, "/api/servers",
		`{"id":"`+testServerID+`","host":"10.0.0.9","tokenName":"panel","tokenSecret":"s"}`, cookie))
	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409: %s", rec.Code, rec.Body.String())
	}
}

// A token never comes back out. The browser proxies every game call through the
// panel, so it has no use for one, and sending it would put it in a tab.
func TestTheTokenIsNeverSentToTheBrowser(t *testing.T) {
	h := newHarness(t, state.Snapshot{})
	cookie := h.login(t, "admin", testPassword)

	if err := h.store.SaveGameServer(t.Context(), gameServerFixture(), h.server.now()); err != nil {
		t.Fatal(err)
	}

	rec := h.do(t, h.request(t, http.MethodGet, "/api/servers", "", cookie))
	if body := rec.Body.String(); containsAny(body, "tokenSecret", "super-secret-value") {
		t.Errorf("the server list leaked a token: %s", body)
	}
}

func containsAny(haystack string, needles ...string) bool {
	for _, n := range needles {
		if len(n) > 0 && len(haystack) > 0 && stringContains(haystack, n) {
			return true
		}
	}
	return false
}

func stringContains(h, n string) bool {
	for i := 0; i+len(n) <= len(h); i++ {
		if h[i:i+len(n)] == n {
			return true
		}
	}
	return false
}

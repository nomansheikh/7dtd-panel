package api

import (
	"context"
	"net/http"

	"github.com/nomansheikh/7dtd-panel/internal/httpx"
	"github.com/nomansheikh/7dtd-panel/internal/servers"
	"github.com/nomansheikh/7dtd-panel/internal/state"
	"github.com/nomansheikh/7dtd-panel/internal/store"
)

type serverCtxKey struct{}

// withServer resolves the {server} path segment once, so no handler has to and
// none can accidentally read a different server's data than the URL names.
func (s *Server) withServer(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("server")
		srv, ok := s.registry.Get(id)
		if !ok {
			httpx.WriteError(w, http.StatusNotFound,
				"no server called "+id+" is configured", "UNKNOWN_SERVER")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), serverCtxKey{}, srv)))
	})
}

// serverFrom returns the game server this request is scoped to. It is only
// called from handlers registered behind withServer, so the value is always
// present.
func serverFrom(ctx context.Context) *servers.Server {
	srv, _ := ctx.Value(serverCtxKey{}).(*servers.Server)
	return srv
}

type serverSummary struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Status lets the switcher show which servers are reachable without the UI
	// having to poll each one's dashboard.
	Status  string `json:"status"`
	Version string `json:"version,omitempty"`
	World   string `json:"world,omitempty"`
	Players int    `json:"players"`
	MaxPlay int    `json:"maxPlayers"`
	// Connect is the address a player would use, once known.
	Connect string `json:"connect,omitempty"`

	// How the panel reaches it, so the edit form can be filled in without a
	// second request. The token's secret is not here and never is: the browser
	// proxies every game call through the panel and has no use for one.
	Host      string `json:"host,omitempty"`
	Port      int    `json:"port,omitempty"`
	Scheme    string `json:"scheme,omitempty"`
	TokenName string `json:"tokenName,omitempty"`
}

// handleServers lists every configured server with enough state for the
// switcher to be useful at a glance.
func (s *Server) handleServers(w http.ResponseWriter, r *http.Request) {
	all := s.registry.All()
	out := make([]serverSummary, 0, len(all))

	// Read once rather than per server: the edit form wants the host and token
	// name, and the registry holds neither in a form worth reaching into.
	stored := map[string]store.GameServer{}
	if saved, err := s.store.GameServers(r.Context()); err == nil {
		for _, g := range saved {
			stored[g.ID] = g
		}
	}

	for _, srv := range all {
		snap := srv.Poller.Snapshot()
		summary := serverSummary{
			ID:      srv.ID,
			Name:    srv.Name,
			Status:  string(snap.Status),
			Version: snap.Version,
			World:   snap.World,
			Players: snap.Stats.Players,
			MaxPlay: snap.MaxPlayers,
			Connect: snap.ConnectAddress,
		}
		if g, ok := stored[srv.ID]; ok {
			summary.Host, summary.Port = g.Host, g.Port
			summary.Scheme, summary.TokenName = g.Scheme, g.TokenName
		}
		if snap.Status == state.StatusUnknown {
			summary.Status = string(state.StatusUnknown)
		}
		out = append(out, summary)
	}

	// A fresh install has none, and this is the first request it makes. An
	// empty default is what tells the UI to offer the setup wizard instead of
	// a switcher.
	defaultID := ""
	if first := s.registry.Default(); first != nil {
		defaultID = first.ID
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"servers": out,
		// The UI opens on this one.
		"default": defaultID,
	})
}

package api

import (
	"context"
	"net/http"
	"time"

	"github.com/nomansheikh/7dtd-panel/internal/config"
	"github.com/nomansheikh/7dtd-panel/internal/httpx"
	"github.com/nomansheikh/7dtd-panel/internal/sdtd"
)

// Trying a game server's details before committing to them. This is the
// endpoint the panel most obviously lacked: until now a mistyped token and a
// switched-off server looked identical, and the only way to try again was to
// edit a compose file and restart the container.

/*
handleTestServer says whether these details would work, before they are saved.

This is the endpoint the panel most obviously lacked. Until now a mistyped
token was indistinguishable from a server that was switched off: both showed as
offline, and the only way to try again was to edit a compose file and restart
the container. Reaching the server and being refused is a different problem
from not reaching it, and saying which is most of the help somebody needs.
*/
func (s *Server) handleTestServer(w http.ResponseWriter, r *http.Request) {
	in, ok := decodeBody[serverInput](w, r)
	if !ok {
		return
	}
	// An edit that left the secret blank means "the one already stored".
	if in.TokenSecret == "" && in.ID != "" {
		if existing, err := s.store.GameServer(r.Context(), in.ID); err == nil {
			in.TokenSecret = existing.TokenSecret
		}
	}
	if in.Host == "" {
		httpx.WriteError(w, http.StatusBadRequest, "the host is required", "INVALID_ARGUMENT")
		return
	}
	if in.Port == 0 {
		in.Port = 8080
	}
	if in.Scheme == "" {
		in.Scheme = "http"
	}

	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	httpx.WriteJSON(w, http.StatusOK, probe(ctx, in.asGame()))
}

// probeResult is what the wizard shows after a test.
type probeResult struct {
	OK bool `json:"ok"`
	// Problem is written for somebody who is mid-setup and does not yet know
	// which half is wrong.
	Problem string `json:"problem,omitempty"`
	// Found is what the server said about itself, so a successful test shows
	// the world's name rather than a bare tick.
	Found *struct {
		Name    string `json:"name,omitempty"`
		World   string `json:"world,omitempty"`
		Version string `json:"version,omitempty"`
		Players int    `json:"players"`
	} `json:"found,omitempty"`
}

func probe(ctx context.Context, gc config.Game) probeResult {
	client, err := sdtd.New(sdtd.Options{
		BaseURL:     gc.BaseURL(),
		TokenName:   gc.TokenName,
		TokenSecret: gc.TokenSecret,
		Timeout:     8 * time.Second,
	})
	if err != nil {
		return probeResult{Problem: err.Error()}
	}

	// Reachability first. serverinfo is public on a default install — it sits
	// at permission level 2000 and answers with no credentials at all — so it
	// says whether the panel can see the server, and nothing about the token.
	info, err := client.ServerInfo(ctx)
	if err != nil {
		return probeResult{Problem: "Could not reach " + gc.BaseURL() + ". " +
			"Check the host and port, and that the Allocs web interface is enabled."}
	}

	// Then the token, against something that actually requires it. Reading one
	// line of the log needs permission level 0–1000, where serverinfo needs
	// none: testing with a public endpoint would call a mistyped secret a
	// success and leave every write to fail later.
	if _, err := client.Log(ctx, 0, 1); err != nil {
		if sdtd.IsUnauthorized(err) {
			return probeResult{Problem: "Reached " + info.Str("GameHost") +
				", but it refused the token. Check the name and secret match a " +
				"`webtokens add` on the game server, and that its level is 0."}
		}
		return probeResult{Problem: "Reached the server, but could not read its log: " + err.Error()}
	}

	out := probeResult{OK: true}
	out.Found = &struct {
		Name    string `json:"name,omitempty"`
		World   string `json:"world,omitempty"`
		Version string `json:"version,omitempty"`
		Players int    `json:"players"`
	}{
		Name:    info.Str("GameHost"),
		World:   info.Str("LevelName"),
		Version: info.Str("ServerVersion"),
	}
	if stats, err := client.ServerStats(ctx); err == nil {
		out.Found.Players = stats.Players
	}
	return out
}

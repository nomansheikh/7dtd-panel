package api

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/nomansheikh/7dtd-panel/internal/config"
	"github.com/nomansheikh/7dtd-panel/internal/httpx"
	"github.com/nomansheikh/7dtd-panel/internal/store"
)

/*
Adding and removing game servers from the panel.

These are the only endpoints that take a game server's token, and the only ones
that are not scoped to a server — they are how a server comes to exist. The
token goes in and never comes back out: the browser has no use for it, since
every game call is proxied.
*/

// serverID is what a server may be called. It ends up in URLs.
var serverID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`)

type serverInput struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Host   string `json:"host"`
	Port   int    `json:"port"`
	Scheme string `json:"scheme"`

	TokenName string `json:"tokenName"`
	// Empty on an edit means "keep the one you have", so the panel never has to
	// send a secret to a browser in order to get it back.
	TokenSecret string `json:"tokenSecret"`
}

// clean fills in the defaults and reports what is wrong, all of it at once.
func (in *serverInput) clean() []string {
	var problems []string

	in.ID = strings.ToLower(strings.TrimSpace(in.ID))
	in.Host = strings.TrimSpace(in.Host)
	in.Name = strings.TrimSpace(in.Name)
	in.TokenName = strings.TrimSpace(in.TokenName)

	if !serverID.MatchString(in.ID) {
		problems = append(problems,
			"the id may use lowercase letters, digits and dashes, start with a letter or digit, "+
				"and run to 32 characters")
	}
	if in.Host == "" {
		problems = append(problems, "the host is required")
	}
	if in.TokenName == "" {
		problems = append(problems, "the token name is required")
	}
	if in.Port == 0 {
		in.Port = 8080
	}
	if in.Port < 1 || in.Port > 65535 {
		problems = append(problems, "the port must be between 1 and 65535")
	}
	if in.Scheme == "" {
		in.Scheme = "http"
	}
	if in.Scheme != "http" && in.Scheme != "https" {
		problems = append(problems, `the scheme must be http or https`)
	}
	if in.Name == "" {
		in.Name = in.Host
	}
	return problems
}

func (in serverInput) asGame() config.Game {
	return config.Game{
		ID: in.ID, Name: in.Name, Host: in.Host, Port: in.Port,
		Scheme: in.Scheme, TokenName: in.TokenName, TokenSecret: in.TokenSecret,
	}
}

// handleCreateServer adds a server and starts it, with no restart.
func (s *Server) handleCreateServer(w http.ResponseWriter, r *http.Request) {
	in, ok := decodeBody[serverInput](w, r)
	if !ok {
		return
	}
	if problems := in.clean(); len(problems) > 0 {
		httpx.WriteError(w, http.StatusBadRequest, strings.Join(problems, "; "), "INVALID_ARGUMENT")
		return
	}
	if in.TokenSecret == "" {
		httpx.WriteError(w, http.StatusBadRequest, "the token secret is required", "INVALID_ARGUMENT")
		return
	}
	if _, exists := s.registry.Get(in.ID); exists {
		httpx.WriteError(w, http.StatusConflict, "a server with that id already exists", "DUPLICATE_ID")
		return
	}

	if err := s.saveAndStart(r.Context(), in, len(s.registry.All())); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"id": in.ID})
}

// handleUpdateServer changes one, restarting its runtime so a new host or token
// takes effect immediately.
func (s *Server) handleUpdateServer(w http.ResponseWriter, r *http.Request) {
	id := strings.ToLower(strings.TrimSpace(r.PathValue("server")))
	in, ok := decodeBody[serverInput](w, r)
	if !ok {
		return
	}
	in.ID = id
	if problems := in.clean(); len(problems) > 0 {
		httpx.WriteError(w, http.StatusBadRequest, strings.Join(problems, "; "), "INVALID_ARGUMENT")
		return
	}
	existing, err := s.store.GameServer(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		httpx.WriteError(w, http.StatusNotFound, "no server with that id", "NOT_FOUND")
		return
	}
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "could not read the server", "STORE_ERROR")
		return
	}

	if err := s.saveAndStart(r.Context(), in, existing.Position); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"id": id})
}

// saveAndStart writes the server and brings its runtime up under the new
// settings, replacing any that was already running for that id.
func (s *Server) saveAndStart(ctx context.Context, in serverInput, position int) error {
	saved := store.GameServer{
		ID: in.ID, Name: in.Name, Host: in.Host, Port: in.Port, Scheme: in.Scheme,
		TokenName: in.TokenName, TokenSecret: in.TokenSecret, Position: position,
	}
	if err := s.store.SaveGameServer(ctx, saved, s.now()); err != nil {
		return err
	}

	// Read back, because a blank secret meant "keep the stored one" and the
	// registry needs the real thing.
	stored, err := s.store.GameServer(ctx, in.ID)
	if err != nil {
		return err
	}
	_, err = s.registry.Add(config.Game{
		ID: stored.ID, Name: stored.Name, Host: stored.Host, Port: stored.Port,
		Scheme: stored.Scheme, TokenName: stored.TokenName, TokenSecret: stored.TokenSecret,
	})
	return err
}

// handleDeleteServer stops a server and forgets it, along with the chat
// commands and tasks that only meant anything for it.
func (s *Server) handleDeleteServer(w http.ResponseWriter, r *http.Request) {
	id := strings.ToLower(strings.TrimSpace(r.PathValue("server")))

	err := s.store.DeleteGameServer(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		httpx.WriteError(w, http.StatusNotFound, "no server with that id", "NOT_FOUND")
		return
	}
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "could not delete the server", "STORE_ERROR")
		return
	}
	s.registry.Remove(id)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

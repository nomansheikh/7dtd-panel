package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/nomansheikh/7dtd-panel/internal/httpx"
)

// eventsBacklog is how much history a newly connected tab receives by default.
const eventsBacklog = 300

// keepAliveEvery bounds how long the connection can sit silent. Proxies and
// load balancers commonly drop idle connections after a minute.
const keepAliveEvery = 25 * time.Second

// handleEvents streams the panel's event feed to one browser client.
//
// The panel holds a single connection to the game server's stream and fans it
// out from memory, so opening ten tabs does not open ten connections to the
// game server, and a tab that arrives late still gets scrollback immediately.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	backlog := eventsBacklog
	if raw := r.URL.Query().Get("backlog"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			httpx.WriteError(w, http.StatusBadRequest,
				"backlog must be a non-negative number", "INVALID_BACKLOG")
			return
		}
		backlog = n
	}

	rc := http.NewResponseController(w)

	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache, no-transform")
	h.Set("Connection", "keep-alive")
	// Tells nginx not to buffer, which would otherwise hold events until the
	// response completed, which for a stream is never.
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	if err := rc.Flush(); err != nil {
		return
	}

	history, ch, cancel := s.events.Subscribe(backlog)
	defer cancel()

	// A write deadline would sever an idle stream, so clear the one the server
	// may have set. If that is not supported, carry on: it only matters when a
	// deadline is actually configured.
	_ = rc.SetWriteDeadline(time.Time{})

	send := func(payload any) bool {
		data, err := json.Marshal(payload)
		if err != nil {
			return true
		}
		if _, err := w.Write([]byte("event: panelEvent\ndata: ")); err != nil {
			return false
		}
		if _, err := w.Write(data); err != nil {
			return false
		}
		if _, err := w.Write([]byte("\n\n")); err != nil {
			return false
		}
		return rc.Flush() == nil
	}

	for _, e := range history {
		if !send(e) {
			return
		}
	}

	keepAlive := time.NewTicker(keepAliveEvery)
	defer keepAlive.Stop()

	for {
		select {
		case <-r.Context().Done():
			return

		case e, ok := <-ch:
			if !ok {
				// The hub dropped this subscriber for falling behind. Closing
				// prompts the browser's EventSource to reconnect and re-seed.
				return
			}
			if !send(e) {
				return
			}

		case <-keepAlive.C:
			if _, err := w.Write([]byte(": keep-alive\n\n")); err != nil {
				return
			}
			if rc.Flush() != nil {
				return
			}
		}
	}
}

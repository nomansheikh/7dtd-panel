package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/nomansheikh/7dtd-panel/internal/httpx"
	"github.com/nomansheikh/7dtd-panel/internal/power"
)

/*
Stopping a game server, and finding out whether it came back.

The sequence runs on the panel rather than in the browser, so closing the tab
part-way through a fifteen-minute countdown does not leave everybody warned and
nothing shut down. These endpoints start it, call it off, and report where it
has got to.
*/

// power is this request's server's controller.
func (s *Server) power(r *http.Request) *power.Controller {
	return serverFrom(r.Context()).Power
}

// countdowns an operator may choose. Bounded rather than free: a countdown
// measured in hours is a scheduled task, and the panel already has those.
const maxCountdownMinutes = 60

func (s *Server) handlePowerStatus(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, s.power(r).Status())
}

func (s *Server) handlePowerStart(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeBody[struct {
		Intent  string `json:"intent"`
		Minutes int    `json:"minutes"`
		Reason  string `json:"reason"`
	}](w, r)
	if !ok {
		return
	}

	intent := power.Intent(req.Intent)
	if intent != power.IntentRestart && intent != power.IntentStop {
		httpx.WriteError(w, http.StatusBadRequest,
			`intent must be "restart" or "stop"`, "INVALID_ARGUMENT")
		return
	}
	if req.Minutes < 0 || req.Minutes > maxCountdownMinutes {
		httpx.WriteError(w, http.StatusBadRequest,
			"a countdown must be between none and an hour", "INVALID_ARGUMENT")
		return
	}
	// The reason is broadcast to players, so it goes through the same check as
	// anything else the panel says on somebody's behalf.
	reason := strings.TrimSpace(req.Reason)
	if strings.ContainsAny(reason, "\"'\r\n") {
		httpx.WriteError(w, http.StatusBadRequest,
			"the reason cannot contain quotes or line breaks", "INVALID_ARGUMENT")
		return
	}

	err := s.power(r).Start(intent, time.Duration(req.Minutes)*time.Minute, reason)
	if errors.Is(err, power.ErrBusy) {
		httpx.WriteError(w, http.StatusConflict, err.Error(), "ALREADY_RUNNING")
		return
	}
	if err != nil {
		httpx.WriteError(w, http.StatusForbidden, err.Error(), "DESTRUCTIVE_BLOCKED")
		return
	}
	httpx.WriteJSON(w, http.StatusAccepted, s.power(r).Status())
}

func (s *Server) handlePowerCancel(w http.ResponseWriter, r *http.Request) {
	if !s.power(r).Cancel() {
		httpx.WriteError(w, http.StatusConflict,
			"there is nothing left to call off", "NOTHING_TO_CANCEL")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, s.power(r).Status())
}

package api

import (
	"net/http"
	"testing"

	"github.com/nomansheikh/7dtd-panel/internal/power"
	"github.com/nomansheikh/7dtd-panel/internal/state"
)

// offPowerController is the same controller with the operator's switch off.
func offPowerController(h *harness) *power.Controller {
	return power.New(power.Options{
		Server: testServerID, Client: h.game, Poller: fakeState{},
		AllowDestructive: false,
	})
}

func TestPowerStartsIdle(t *testing.T) {
	h := newHarness(t, state.Snapshot{Status: state.StatusOnline})
	cookie := h.login(t, "admin", testPassword)

	rec := h.do(t, h.request(t, http.MethodGet, "/api/power", "", cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if got := decode[map[string]any](t, rec)["phase"]; got != "idle" {
		t.Errorf("phase = %v, want idle", got)
	}
}

func TestPowerRejectsWhatCannotWork(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"no intent", `{"minutes":5}`},
		{"an intent that is neither", `{"intent":"reboot","minutes":5}`},
		{"a countdown of a day", `{"intent":"restart","minutes":1440}`},
		{"a negative countdown", `{"intent":"restart","minutes":-1}`},
		// The reason is broadcast, so it goes through the same check as
		// anything else the panel says on somebody's behalf.
		{"a reason with a quote in it", `{"intent":"restart","minutes":1,"reason":"say \"hi\""}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, state.Snapshot{Status: state.StatusOnline})
			cookie := h.login(t, "admin", testPassword)
			rec := h.do(t, h.request(t, http.MethodPost, "/api/power", tc.body, cookie))
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

// PANEL_ALLOW_DESTRUCTIVE is the operator's switch, and a countdown does not
// get around it.
func TestPowerHonoursTheDestructiveSwitch(t *testing.T) {
	h := newHarness(t, state.Snapshot{Status: state.StatusOnline})
	h.srv.Power = offPowerController(h)
	cookie := h.login(t, "admin", testPassword)

	rec := h.do(t, h.request(t, http.MethodPost, "/api/power",
		`{"intent":"restart","minutes":0}`, cookie))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403: %s", rec.Code, rec.Body.String())
	}
	for _, c := range h.game.executed {
		if c == "shutdown" {
			t.Error("stopped the server with the switch off")
		}
	}
}

// Nothing to call off is a conflict, not a lie about having cancelled.
func TestCancellingNothing(t *testing.T) {
	h := newHarness(t, state.Snapshot{Status: state.StatusOnline})
	cookie := h.login(t, "admin", testPassword)

	rec := h.do(t, h.request(t, http.MethodPost, "/api/power/cancel", "", cookie))
	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409: %s", rec.Code, rec.Body.String())
	}
}

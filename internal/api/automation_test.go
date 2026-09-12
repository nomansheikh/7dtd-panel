package api

import (
	"net/http"
	"strings"
	"testing"

	"github.com/nomansheikh/7dtd-panel/internal/state"
)

func taskHarness(t *testing.T) (*harness, *http.Cookie) {
	t.Helper()
	h := newHarness(t, state.Snapshot{Status: state.StatusOnline})
	return h, h.login(t, "admin", testPassword)
}

func (h *harness) tasks(t *testing.T, cookie *http.Cookie) []taskRow {
	t.Helper()
	rec := h.do(t, h.request(t, http.MethodGet, "/api/tasks", "", cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("listing returned %d: %s", rec.Code, rec.Body.String())
	}
	return decode[struct {
		Tasks []taskRow `json:"tasks"`
	}](t, rec).Tasks
}

// An empty history is a list of nothing. Returned as null it crashed the page
// that had been promised an array.
func TestAnEmptyHistoryIsAnEmptyList(t *testing.T) {
	h, cookie := taskHarness(t)
	rec := h.do(t, h.request(t, http.MethodGet, "/api/tasks", "", cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, `"runs":null`) || strings.Contains(body, `"tasks":null`) {
		t.Errorf("a list came back as null: %s", body)
	}
}

func TestATaskRoundTripsWithItsTierReported(t *testing.T) {
	h, cookie := taskHarness(t)

	rec := h.do(t, h.request(t, http.MethodPut, "/api/tasks/restart",
		`{"enabled":true,"trigger":"daily","at":"05:00","description":"Nightly restart",
		  "commands":["say \"restarting in 1 minute\"","saveworld","shutdown"]}`, cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}

	for _, task := range h.tasks(t, cookie) {
		if task.Name != "restart" {
			continue
		}
		if task.Trigger != "daily" || task.At != "05:00" {
			t.Errorf("trigger came back as %+v", task)
		}
		if len(task.Commands) != 3 {
			t.Fatalf("lines = %+v", task.Commands)
		}
		// An operator switching on a nightly restart should see that the last
		// line of it is a shutdown.
		if task.Commands[2].Tier != "destructive" || task.Tier != "destructive" {
			t.Errorf("tiers = %q / %q, want destructive", task.Commands[2].Tier, task.Tier)
		}
		return
	}
	t.Fatal("the task was not in the list")
}

func TestTaskRejections(t *testing.T) {
	tests := []struct {
		name string
		path string
		body string
	}{
		{
			name: "no commands",
			path: "/api/tasks/empty",
			body: `{"enabled":true,"trigger":"every","minutes":30,"commands":[]}`,
		},
		{
			name: "a daily task with no time",
			path: "/api/tasks/restart",
			body: `{"enabled":true,"trigger":"daily","commands":["saveworld"]}`,
		},
		{
			name: "a daily task with a nonsense time",
			path: "/api/tasks/restart",
			body: `{"enabled":true,"trigger":"daily","at":"5pm","commands":["saveworld"]}`,
		},
		{
			name: "an interval of zero",
			path: "/api/tasks/save",
			body: `{"enabled":true,"trigger":"every","minutes":0,"commands":["saveworld"]}`,
		},
		{
			name: "an unknown trigger",
			path: "/api/tasks/save",
			body: `{"enabled":true,"trigger":"whenever","commands":["saveworld"]}`,
		},
		{
			name: "a name that could not be typed",
			path: "/api/tasks/My%20Task",
			body: `{"enabled":true,"trigger":"every","minutes":30,"commands":["saveworld"]}`,
		},
		{
			// A second command on one line would never appear in what the
			// operator reviewed.
			name: "two commands on one line",
			path: "/api/tasks/sneaky",
			body: `{"enabled":true,"trigger":"every","minutes":30,` +
				`"commands":["say hello\nshutdown"]}`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h, cookie := taskHarness(t)
			rec := h.do(t, h.request(t, http.MethodPut, tc.path, tc.body, cookie))
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestATaskCanBeDeleted(t *testing.T) {
	h, cookie := taskHarness(t)
	rec := h.do(t, h.request(t, http.MethodPut, "/api/tasks/save",
		`{"enabled":true,"trigger":"every","minutes":30,"commands":["saveworld"]}`, cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}

	rec = h.do(t, h.request(t, http.MethodDelete, "/api/tasks/save", "", cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("delete returned %d: %s", rec.Code, rec.Body.String())
	}
	rec = h.do(t, h.request(t, http.MethodDelete, "/api/tasks/save", "", cookie))
	if rec.Code != http.StatusNotFound {
		t.Errorf("deleting twice returned %d, want 404", rec.Code)
	}
}

// Tasks belong to one server, like everything else the panel keeps.
func TestTasksAreScopedToTheirServer(t *testing.T) {
	h, cookie := taskHarness(t)
	rec := h.do(t, h.request(t, http.MethodPut, "/api/tasks/save",
		`{"enabled":true,"trigger":"every","minutes":30,"commands":["saveworld"]}`, cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}

	other, err := h.store.Tasks(t.Context(), "somewhere-else")
	if err != nil {
		t.Fatal(err)
	}
	if len(other) != 0 {
		t.Errorf("another server sees %+v", other)
	}
}

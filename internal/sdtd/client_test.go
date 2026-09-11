package sdtd

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeServer stands in for a 7 Days to Die server. Every fixture it serves was
// captured from a real one, so the tests exercise the actual envelope shapes,
// the quoted uptime field and the real meta.errorCode values rather than
// invented ones.
type fakeServer struct {
	*httptest.Server
	// requests records what the client actually sent.
	requests []*http.Request
}

type route struct {
	status  int
	fixture string
	body    string
}

func newFakeServer(t *testing.T, routes map[string]route) *fakeServer {
	t.Helper()
	fs := &fakeServer{}
	mux := http.NewServeMux()
	for pattern, r := range routes {
		r := r
		mux.HandleFunc(pattern, func(w http.ResponseWriter, req *http.Request) {
			fs.requests = append(fs.requests, req)
			status := r.status
			if status == 0 {
				status = http.StatusOK
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			if r.fixture != "" {
				b, err := os.ReadFile(filepath.Join("testdata", r.fixture))
				if err != nil {
					t.Errorf("read fixture %s: %v", r.fixture, err)
					return
				}
				_, _ = w.Write(b)
				return
			}
			_, _ = w.Write([]byte(r.body))
		})
	}
	fs.Server = httptest.NewServer(mux)
	t.Cleanup(fs.Close)
	return fs
}

func newTestClient(t *testing.T, fs *fakeServer) *Client {
	t.Helper()
	c, err := New(Options{
		BaseURL:     fs.URL,
		TokenName:   "panel",
		TokenSecret: "test-secret",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

func TestNewValidatesOptions(t *testing.T) {
	tests := []struct {
		name string
		opts Options
	}{
		{"no base URL", Options{TokenName: "panel", TokenSecret: "s"}},
		{"no token name", Options{BaseURL: "http://x", TokenSecret: "s"}},
		{"no token secret", Options{BaseURL: "http://x", TokenName: "panel"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := New(tt.opts); err == nil {
				t.Error("New succeeded, want error")
			}
		})
	}
}

func TestClientSendsBothAuthHeaders(t *testing.T) {
	fs := newFakeServer(t, map[string]route{
		"/api/serverstats": {fixture: "serverstats.json"},
	})
	c := newTestClient(t, fs)

	if _, err := c.ServerStats(context.Background()); err != nil {
		t.Fatalf("ServerStats: %v", err)
	}
	if len(fs.requests) != 1 {
		t.Fatalf("got %d requests, want 1", len(fs.requests))
	}
	req := fs.requests[0]

	// Both headers are required together; sending one is an auth failure.
	if got := req.Header.Get(HeaderTokenName); got != "panel" {
		t.Errorf("%s = %q, want panel", HeaderTokenName, got)
	}
	if got := req.Header.Get(HeaderTokenSecret); got != "test-secret" {
		t.Errorf("%s = %q, want test-secret", HeaderTokenSecret, got)
	}
}

func TestServerStats(t *testing.T) {
	fs := newFakeServer(t, map[string]route{
		"/api/serverstats": {fixture: "serverstats.json"},
	})
	c := newTestClient(t, fs)

	stats, err := c.ServerStats(context.Background())
	if err != nil {
		t.Fatalf("ServerStats: %v", err)
	}
	// The fixture is a real capture: day 1, 07:00, empty server.
	if stats.GameTime.Days != 1 {
		t.Errorf("days = %d, want 1", stats.GameTime.Days)
	}
	if stats.GameTime.Hours != 7 {
		t.Errorf("hours = %d, want 7", stats.GameTime.Hours)
	}
	if stats.Players != 0 || stats.Hostiles != 0 || stats.Animals != 0 {
		t.Errorf("counts = %d/%d/%d, want all zero", stats.Players, stats.Hostiles, stats.Animals)
	}
}

func TestServerInfoFlattensTypedValues(t *testing.T) {
	fs := newFakeServer(t, map[string]route{
		"/api/serverinfo": {fixture: "serverinfo.json"},
	})
	c := newTestClient(t, fs)

	info, err := c.ServerInfo(context.Background())
	if err != nil {
		t.Fatalf("ServerInfo: %v", err)
	}
	if info.Len() == 0 {
		t.Fatal("no values decoded")
	}

	tests := []struct {
		name  string
		check func(t *testing.T)
	}{
		{"string value", func(t *testing.T) {
			if got := info.Str("GameType"); got != "7DTD" {
				t.Errorf("GameType = %q, want 7DTD", got)
			}
		}},
		{"int value", func(t *testing.T) {
			if got := info.Int("MaxPlayers"); got != 8 {
				t.Errorf("MaxPlayers = %d, want 8", got)
			}
		}},
		{"version is present", func(t *testing.T) {
			// Note this is the server's own mangled rendering. The console
			// version command reports "V 3.2.0 (b10)" for the same build.
			if got := info.Str("ServerVersion"); got == "" {
				t.Error("ServerVersion is empty")
			}
		}},
		{"absent name yields zero values", func(t *testing.T) {
			if _, ok := info.Get("NoSuchField"); ok {
				t.Error("Get reported a missing field as present")
			}
			if got := info.Str("NoSuchField"); got != "" {
				t.Errorf("Str = %q, want empty", got)
			}
			if got := info.Int("NoSuchField"); got != 0 {
				t.Errorf("Int = %d, want 0", got)
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, tt.check)
	}
}

func TestLogParsesQuotedUptime(t *testing.T) {
	fs := newFakeServer(t, map[string]route{
		"/api/log": {fixture: "log_page.json"},
	})
	c := newTestClient(t, fs)

	page, err := c.Log(context.Background(), -1, 3)
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	if len(page.Entries) == 0 {
		t.Fatal("no entries decoded")
	}
	if page.LastLine != 3 {
		t.Errorf("lastLine = %d, want 3", page.LastLine)
	}

	// uptime arrives as a JSON string, not a number. This is the only source
	// of server uptime anywhere in the API, so parsing it matters.
	first := page.Entries[0]
	d, ok := first.UptimeDuration()
	if !ok {
		t.Fatalf("UptimeDuration failed for %q", first.Uptime)
	}
	if want := 41831 * time.Millisecond; d != want {
		t.Errorf("uptime = %s, want %s", d, want)
	}
	if _, ok := first.Time(); !ok {
		t.Errorf("Time failed for %q", first.ISOTime)
	}
}

func TestLogSendsCursorParams(t *testing.T) {
	fs := newFakeServer(t, map[string]route{
		"/api/log": {fixture: "log_page.json"},
	})
	c := newTestClient(t, fs)

	if _, err := c.Log(context.Background(), 42, -10); err != nil {
		t.Fatalf("Log: %v", err)
	}
	q := fs.requests[0].URL.Query()
	if got := q.Get("firstLine"); got != "42" {
		t.Errorf("firstLine = %q, want 42", got)
	}
	if got := q.Get("count"); got != "-10" {
		t.Errorf("count = %q, want -10", got)
	}
}

func TestLogOmitsNegativeFirstLine(t *testing.T) {
	fs := newFakeServer(t, map[string]route{
		"/api/log": {fixture: "log_page.json"},
	})
	c := newTestClient(t, fs)

	if _, err := c.Log(context.Background(), -1, 50); err != nil {
		t.Fatalf("Log: %v", err)
	}
	if q := fs.requests[0].URL.Query(); q.Has("firstLine") {
		t.Errorf("firstLine should be omitted so the server picks, got %q", q.Get("firstLine"))
	}
}

func TestExecuteReturnsCommandResult(t *testing.T) {
	fs := newFakeServer(t, map[string]route{
		"/api/command": {fixture: "command_version.json"},
	})
	c := newTestClient(t, fs)

	res, err := c.Execute(context.Background(), "version")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Command != "version" {
		t.Errorf("command = %q, want version", res.Command)
	}
	if res.Result == "" {
		t.Error("result is empty")
	}
	if fs.requests[0].Method != http.MethodPost {
		t.Errorf("method = %s, want POST", fs.requests[0].Method)
	}
}

func TestErrorMapping(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		fixture   string
		body      string
		wantCode  string
		wantFound bool
		wantAuth  bool
	}{
		{
			name:     "unknown command is 404 with a code",
			status:   http.StatusNotFound,
			fixture:  "error_unknown_command.json",
			wantCode: CodeUnknownCommand,
			// 404 here means the command does not exist, not the endpoint.
			wantFound: true,
		},
		{
			name:     "empty body is 400 with a code",
			status:   http.StatusBadRequest,
			fixture:  "error_no_command.json",
			wantCode: CodeNoCommand,
		},
		{
			name:     "403 is treated as an auth failure",
			status:   http.StatusForbidden,
			body:     `{"data":[],"meta":{"errorCode":""}}`,
			wantAuth: true,
		},
		{
			name:     "500 with an exception keeps the message",
			status:   http.StatusInternalServerError,
			body:     `{"data":[],"meta":{"errorCode":"INVALID_BODY","exceptionMessage":"boom","exceptionTrace":"at Foo()"}}`,
			wantCode: "INVALID_BODY",
		},
		{
			name:   "non-JSON error body still yields an APIError",
			status: http.StatusBadGateway,
			body:   "upstream exploded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := newFakeServer(t, map[string]route{
				"/api/command": {status: tt.status, fixture: tt.fixture, body: tt.body},
			})
			c := newTestClient(t, fs)

			_, err := c.Execute(context.Background(), "whatever")
			if err == nil {
				t.Fatal("Execute succeeded, want error")
			}

			var apiErr *APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("error is %T, want *APIError", err)
			}
			if apiErr.Status != tt.status {
				t.Errorf("status = %d, want %d", apiErr.Status, tt.status)
			}
			if tt.wantCode != "" {
				if apiErr.ErrorCode != tt.wantCode {
					t.Errorf("errorCode = %q, want %q", apiErr.ErrorCode, tt.wantCode)
				}
				if got := ErrorCode(err); got != tt.wantCode {
					t.Errorf("ErrorCode(err) = %q, want %q", got, tt.wantCode)
				}
			}
			if got := IsNotFound(err); got != tt.wantFound {
				t.Errorf("IsNotFound = %v, want %v", got, tt.wantFound)
			}
			if got := IsUnauthorized(err); got != tt.wantAuth {
				t.Errorf("IsUnauthorized = %v, want %v", got, tt.wantAuth)
			}
			// The trace must never reach a browser.
			if apiErr.Trace != "" && strings.Contains(apiErr.SafeMessage(), apiErr.Trace) {
				t.Error("SafeMessage leaks the stack trace")
			}
		})
	}
}

func TestTwoHundredWithErrorCodeIsAnError(t *testing.T) {
	// Some endpoints answer generically and can return 200 alongside an
	// errorCode. Returning zero-valued data there would be silently wrong.
	fs := newFakeServer(t, map[string]route{
		"/api/serverstats": {
			status: http.StatusOK,
			body:   `{"data":{},"meta":{"errorCode":"Unsupported"}}`,
		},
	})
	c := newTestClient(t, fs)

	if _, err := c.ServerStats(context.Background()); err == nil {
		t.Fatal("ServerStats succeeded, want error")
	} else if got := ErrorCode(err); got != CodeUnsupported {
		t.Errorf("ErrorCode = %q, want %q", got, CodeUnsupported)
	}
}

func TestMalformedJSONIsReported(t *testing.T) {
	fs := newFakeServer(t, map[string]route{
		"/api/serverstats": {body: "{not json"},
	})
	c := newTestClient(t, fs)

	_, err := c.ServerStats(context.Background())
	if err == nil {
		t.Fatal("ServerStats succeeded, want error")
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		t.Error("a decode failure should not masquerade as an APIError")
	}
}

func TestUnreachableServerReturnsError(t *testing.T) {
	// The panel must survive the game server being down, so this path has to
	// produce a plain error and never panic.
	c, err := New(Options{
		BaseURL:     "http://127.0.0.1:1",
		TokenName:   "panel",
		TokenSecret: "s",
		Timeout:     200 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := c.ServerStats(context.Background()); err == nil {
		t.Fatal("ServerStats succeeded against a closed port")
	}
}

func TestContextCancellationIsHonoured(t *testing.T) {
	fs := newFakeServer(t, map[string]route{
		"/api/serverstats": {fixture: "serverstats.json"},
	})
	c := newTestClient(t, fs)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := c.ServerStats(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
}

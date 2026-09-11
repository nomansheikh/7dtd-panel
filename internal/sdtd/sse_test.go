package sdtd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// sseServer is a controllable stand-in for the game server's log stream.
type sseServer struct {
	*httptest.Server

	mu sync.Mutex
	// logPages is what /api/log returns, keyed by the firstLine requested.
	// A nil entry means "serve from the entries slice".
	entries   []LogEntry
	lastLine  int
	logCalls  []string
	connects  int
	frames    chan string
	closeConn chan struct{}
}

func newSSEServer(t *testing.T) *sseServer {
	t.Helper()
	s := &sseServer{
		frames:    make(chan string, 64),
		closeConn: make(chan struct{}),
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/sse/", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.connects++
		// Captured under the lock: drop() swaps this channel, and reading it
		// from the select below without synchronising is a data race.
		closeCh := s.closeConn
		s.mu.Unlock()

		if r.URL.Query().Get("events") != "log" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		rc := http.NewResponseController(w)
		_ = rc.Flush()

		for {
			select {
			case <-r.Context().Done():
				return
			case <-closeCh:
				return
			case frame, ok := <-s.frames:
				if !ok {
					return
				}
				_, _ = io.WriteString(w, frame)
				_ = rc.Flush()
			}
		}
	})

	mux.HandleFunc("/api/log", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		first := r.URL.Query().Get("firstLine")
		count := r.URL.Query().Get("count")
		s.logCalls = append(s.logCalls, "firstLine="+first+",count="+count)

		var out []LogEntry
		if first == "" {
			// Negative count: the newest N lines.
			n, _ := strconv.Atoi(count)
			if n < 0 {
				n = -n
			}
			if n > len(s.entries) {
				n = len(s.entries)
			}
			out = s.entries[len(s.entries)-n:]
		} else {
			from, _ := strconv.Atoi(first)
			for _, e := range s.entries {
				if e.ID >= from {
					out = append(out, e)
				}
			}
		}
		page := LogPage{Entries: out, LastLine: s.lastLine}
		if len(out) > 0 {
			page.FirstLine = out[0].ID
		}
		s.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": page, "meta": map[string]any{}})
	})

	s.Server = httptest.NewServer(mux)
	t.Cleanup(s.Close)
	return s
}

// setLog replaces what /api/log will serve.
func (s *sseServer) setLog(entries []LogEntry, lastLine int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = entries
	s.lastLine = lastLine
}

// push queues a logLine frame onto the live stream.
func (s *sseServer) push(e LogEntry) {
	b, _ := json.Marshal(e)
	s.frames <- fmt.Sprintf("event: %s\ndata: %s\n\n", sseEventName, b)
}

// pushRaw queues an arbitrary frame.
func (s *sseServer) pushRaw(frame string) { s.frames <- frame }

// drop severs the current connection and arms a fresh close channel for the
// next one.
func (s *sseServer) drop() {
	s.mu.Lock()
	current := s.closeConn
	s.closeConn = make(chan struct{})
	s.mu.Unlock()
	close(current)
}

func (s *sseServer) connectCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.connects
}

func entry(id int, msg string) LogEntry {
	return LogEntry{ID: id, Msg: msg, Type: "Log", Uptime: "1000", ISOTime: "2026-09-11T08:00:00.0000000+00:00"}
}

// collector gathers emitted entries in order.
type collector struct {
	mu      sync.Mutex
	entries []LogEntry
	notify  chan struct{}
}

func newCollector() *collector {
	return &collector{notify: make(chan struct{}, 256)}
}

func (c *collector) emit(e LogEntry) {
	c.mu.Lock()
	c.entries = append(c.entries, e)
	c.mu.Unlock()
	select {
	case c.notify <- struct{}{}:
	default:
	}
}

func (c *collector) ids() []int {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]int, len(c.entries))
	for i, e := range c.entries {
		out[i] = e.ID
	}
	return out
}

// waitFor blocks until n entries have arrived or the deadline passes.
func (c *collector) waitFor(t *testing.T, n int, within time.Duration) {
	t.Helper()
	deadline := time.After(within)
	for {
		c.mu.Lock()
		got := len(c.entries)
		c.mu.Unlock()
		if got >= n {
			return
		}
		select {
		case <-c.notify:
		case <-deadline:
			t.Fatalf("timed out waiting for %d entries; got %d: %v", n, got, c.ids())
		}
	}
}

func newTestStreamer(t *testing.T, s *sseServer) *LogStreamer {
	t.Helper()
	c, err := New(Options{
		BaseURL:     s.URL,
		TokenName:   "panel",
		TokenSecret: "secret",
		Timeout:     5 * time.Second,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	st := NewLogStreamer(c, slog.New(slog.NewTextHandler(io.Discard, nil)))
	// Keep reconnects quick so tests do not crawl.
	st.InitialBackoff = 10 * time.Millisecond
	st.MaxBackoff = 40 * time.Millisecond
	return st
}

func TestStreamSeedsThenDeliversLiveEntries(t *testing.T) {
	s := newSSEServer(t)
	s.setLog([]LogEntry{entry(1, "one"), entry(2, "two")}, 3)

	st := newTestStreamer(t, s)
	c := newCollector()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go st.Run(ctx, c.emit)

	// Cold start pulls recent history first.
	c.waitFor(t, 2, 3*time.Second)

	s.push(entry(3, "three"))
	c.waitFor(t, 3, 3*time.Second)

	want := []int{1, 2, 3}
	if got := c.ids(); !equalInts(got, want) {
		t.Errorf("ids = %v, want %v", got, want)
	}
}

func TestStreamSendsAuthHeadersAndCorrectPath(t *testing.T) {
	var (
		mu   sync.Mutex
		seen *http.Request
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/sse") {
			mu.Lock()
			seen = r.Clone(context.Background())
			mu.Unlock()
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c, err := New(Options{BaseURL: srv.URL, TokenName: "panel", TokenSecret: "secret"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	st := NewLogStreamer(c, slog.New(slog.NewTextHandler(io.Discard, nil)))

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	_ = st.runOnce(ctx, new(int), func(LogEntry) {})

	mu.Lock()
	defer mu.Unlock()
	if seen == nil {
		t.Fatal("the stream endpoint was never called")
	}
	if got := seen.URL.Query().Get("events"); got != "log" {
		t.Errorf("events param = %q, want log", got)
	}
	if got := seen.Header.Get(HeaderTokenName); got != "panel" {
		t.Errorf("%s = %q", HeaderTokenName, got)
	}
	if got := seen.Header.Get(HeaderTokenSecret); got != "secret" {
		t.Errorf("%s = %q", HeaderTokenSecret, got)
	}
	if got := seen.Header.Get("Accept"); got != "text/event-stream" {
		t.Errorf("Accept = %q", got)
	}
}

// TestReconnectBackfillsTheGap is the reason this type exists: the stream has
// no replay, so lines written while disconnected must come from /api/log.
func TestReconnectBackfillsTheGap(t *testing.T) {
	s := newSSEServer(t)
	s.setLog([]LogEntry{entry(1, "one")}, 2)

	st := newTestStreamer(t, s)
	c := newCollector()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go st.Run(ctx, c.emit)

	c.waitFor(t, 1, 3*time.Second)

	// While disconnected the server writes 2, 3 and 4.
	s.setLog([]LogEntry{
		entry(1, "one"), entry(2, "two"), entry(3, "three"), entry(4, "four"),
	}, 5)
	s.drop()

	// Those three must arrive via backfill, in order, exactly once.
	c.waitFor(t, 4, 5*time.Second)
	want := []int{1, 2, 3, 4}
	if got := c.ids(); !equalInts(got, want) {
		t.Errorf("ids = %v, want %v (gaps or duplicates)", got, want)
	}
}

// TestLiveEntriesDoNotOvertakeBackfill guards the ordering hazard: if a live
// entry were emitted before the backfill ran, the cursor would jump past the
// older lines and they would be dropped as duplicates.
func TestLiveEntriesDoNotOvertakeBackfill(t *testing.T) {
	s := newSSEServer(t)
	// The log already holds 1..5; the live stream will immediately push 6.
	var existing []LogEntry
	for i := 1; i <= 5; i++ {
		existing = append(existing, entry(i, fmt.Sprintf("line %d", i)))
	}
	s.setLog(existing, 6)

	// Queue the live frame before the streamer even connects, so it is read
	// as early as possible relative to the backfill.
	s.push(entry(6, "six"))

	st := newTestStreamer(t, s)
	c := newCollector()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go st.Run(ctx, c.emit)

	c.waitFor(t, 6, 5*time.Second)

	want := []int{1, 2, 3, 4, 5, 6}
	if got := c.ids(); !equalInts(got, want) {
		t.Errorf("ids = %v, want %v; a live entry overtook the backfill", got, want)
	}
}

func TestDuplicatesAreSuppressed(t *testing.T) {
	s := newSSEServer(t)
	s.setLog([]LogEntry{entry(1, "one")}, 2)

	st := newTestStreamer(t, s)
	c := newCollector()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go st.Run(ctx, c.emit)

	c.waitFor(t, 1, 3*time.Second)

	// The server re-sends a line already delivered, then a genuinely new one.
	s.push(entry(1, "one again"))
	s.push(entry(2, "two"))
	c.waitFor(t, 2, 3*time.Second)

	if got := c.ids(); !equalInts(got, []int{1, 2}) {
		t.Errorf("ids = %v, want [1 2]; a duplicate was delivered", got)
	}
}

// TestLogRestartResetsCursor covers the game server restarting, which sends its
// log ids back to zero. Without detection every later line looks like a
// duplicate and the feed goes permanently silent.
func TestLogRestartResetsCursor(t *testing.T) {
	s := newSSEServer(t)
	var many []LogEntry
	for i := 90; i <= 99; i++ {
		many = append(many, entry(i, fmt.Sprintf("old %d", i)))
	}
	s.setLog(many, 100)

	st := newTestStreamer(t, s)
	c := newCollector()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go st.Run(ctx, c.emit)

	c.waitFor(t, 10, 3*time.Second)

	// Server restarts: the log is now short and starts again at 0.
	s.setLog([]LogEntry{entry(0, "StartGame done"), entry(1, "fresh")}, 2)
	s.drop()

	c.waitFor(t, 12, 5*time.Second)

	got := c.ids()
	tail := got[len(got)-2:]
	if !equalInts(tail, []int{0, 1}) {
		t.Errorf("entries after restart = %v, want the feed to resume at [0 1]", tail)
	}
}

func TestMalformedFrameDoesNotKillTheStream(t *testing.T) {
	s := newSSEServer(t)
	s.setLog([]LogEntry{entry(1, "one")}, 2)

	st := newTestStreamer(t, s)
	c := newCollector()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go st.Run(ctx, c.emit)
	c.waitFor(t, 1, 3*time.Second)

	before := s.connectCount()
	s.pushRaw("event: logLine\ndata: {not json\n\n")
	s.pushRaw(": keep-alive comment\n\n")
	s.pushRaw("event: somethingElse\ndata: {\"id\":999}\n\n")
	s.push(entry(2, "two"))

	c.waitFor(t, 2, 3*time.Second)
	if got := c.ids(); !equalInts(got, []int{1, 2}) {
		t.Errorf("ids = %v, want [1 2]", got)
	}
	if after := s.connectCount(); after != before {
		t.Errorf("stream reconnected %d times; a bad frame should not drop it", after-before)
	}
}

func TestNonOKResponseIsAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	c, _ := New(Options{BaseURL: srv.URL, TokenName: "p", TokenSecret: "s"})
	st := NewLogStreamer(c, slog.New(slog.NewTextHandler(io.Discard, nil)))

	err := st.runOnce(context.Background(), new(int), func(LogEntry) {})
	if err == nil {
		t.Fatal("runOnce succeeded against a 403")
	}
	var apiErr *APIError
	if !asAPI(err, &apiErr) || apiErr.Status != http.StatusForbidden {
		t.Errorf("error = %v, want an APIError with status 403", err)
	}
}

func TestWrongContentTypeIsAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c, _ := New(Options{BaseURL: srv.URL, TokenName: "p", TokenSecret: "s"})
	st := NewLogStreamer(c, slog.New(slog.NewTextHandler(io.Discard, nil)))

	if err := st.runOnce(context.Background(), new(int), func(LogEntry) {}); err == nil {
		t.Fatal("runOnce accepted a non-event-stream response")
	}
}

func TestRunStopsOnContextCancel(t *testing.T) {
	s := newSSEServer(t)
	s.setLog(nil, 0)
	st := newTestStreamer(t, s)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		st.Run(ctx, func(LogEntry) {})
		close(done)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not return after cancellation")
	}
}

func TestScanEventsParsing(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []int
	}{
		{
			name:  "single event",
			input: "event: logLine\ndata: {\"id\":1}\n\n",
			want:  []int{1},
		},
		{
			name:  "several events",
			input: "event: logLine\ndata: {\"id\":1}\n\nevent: logLine\ndata: {\"id\":2}\n\n",
			want:  []int{1, 2},
		},
		{
			name:  "data without a space after the colon",
			input: "event: logLine\ndata:{\"id\":7}\n\n",
			want:  []int{7},
		},
		{
			name:  "comments and keep-alives are ignored",
			input: ": ping\n\nevent: logLine\ndata: {\"id\":3}\n\n",
			want:  []int{3},
		},
		{
			name:  "other event names are ignored",
			input: "event: chat\ndata: {\"id\":4}\n\n",
			want:  nil,
		},
		{
			name:  "id and retry fields are ignored",
			input: "id: 99\nretry: 5000\nevent: logLine\ndata: {\"id\":5}\n\n",
			want:  []int{5},
		},
		{
			name:  "a frame with no terminating blank line is not emitted",
			input: "event: logLine\ndata: {\"id\":6}\n",
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []int
			err := scanEvents(strings.NewReader(tt.input), func(e LogEntry) {
				got = append(got, e.ID)
			})
			if err != nil {
				t.Fatalf("scanEvents: %v", err)
			}
			if !equalInts(got, tt.want) {
				t.Errorf("ids = %v, want %v", got, tt.want)
			}
		})
	}
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

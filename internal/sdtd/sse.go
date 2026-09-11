package sdtd

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"strings"
	"sync"
	"time"
)

// The server streams its log over Server-Sent Events. This endpoint appears
// nowhere in the published OpenAPI spec; it was found by noticing that /sse/log
// answers 400 rather than 404 and then reading the call site out of the
// server's own bundled web app:
//
//	GET /sse/?events=log  ->  event: logLine
//	                          data:  {"id":61,"msg":"…","type":"Log",…}
//
// Only "log" is a valid event name; chat, playerlist and players all return 400.
// Chat therefore arrives as ordinary log lines.
const (
	ssePath      = "/sse/?events=log"
	sseEventName = "logLine"
)

// seedLines is how much history to pull on a cold start, so a freshly opened
// panel shows recent context instead of an empty feed.
const seedLines = 200

// backfillLines caps how much is fetched to close a reconnect gap.
const backfillLines = 500

// LogStreamer delivers log entries in order, without gaps or duplicates,
// across reconnects.
//
// The stream itself has no replay: reconnecting starts from "now", so anything
// written while disconnected would be lost. /api/log takes a firstLine cursor,
// so every connection is paired with a backfill of the gap. Entries arriving on
// the stream during that backfill are buffered, because emitting them first
// would advance the cursor past the very lines being fetched.
type LogStreamer struct {
	client *Client
	log    *slog.Logger

	// Backoff bounds for reconnection.
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
}

// NewLogStreamer builds a streamer over an existing client.
func NewLogStreamer(c *Client, log *slog.Logger) *LogStreamer {
	if log == nil {
		log = slog.Default()
	}
	return &LogStreamer{
		client:         c,
		log:            log,
		InitialBackoff: time.Second,
		MaxBackoff:     30 * time.Second,
	}
}

// Run streams until ctx is cancelled, calling emit for each new entry.
//
// emit is called from a single goroutine, so implementations do not need their
// own locking, but it must not block for long or it will stall the stream.
func (s *LogStreamer) Run(ctx context.Context, emit func(LogEntry)) {
	// -1 rather than 0 because the server's first log line has id 0.
	lastSeen := -1
	backoff := s.InitialBackoff

	for {
		if ctx.Err() != nil {
			return
		}

		err := s.runOnce(ctx, &lastSeen, emit)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			s.log.Warn("log stream disconnected", "error", err, "retryIn", backoff)
		} else {
			s.log.Info("log stream closed by the server", "retryIn", backoff)
		}

		// Jitter keeps a fleet of panels from reconnecting in lockstep after a
		// game server restart.
		wait := backoff + time.Duration(rand.Int64N(int64(backoff/2)+1))
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}

		if backoff < s.MaxBackoff {
			backoff *= 2
			if backoff > s.MaxBackoff {
				backoff = s.MaxBackoff
			}
		}
	}
}

// runOnce holds one connection open until it fails or ctx ends.
func (s *LogStreamer) runOnce(ctx context.Context, lastSeen *int, emit func(LogEntry)) error {
	resp, err := s.open(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
		_ = resp.Body.Close()
	}()

	var (
		mu         sync.Mutex
		buffered   []LogEntry
		backfilled bool
	)

	// deliver is the only place lastSeen moves, so ordering and de-duplication
	// are decided in one spot.
	deliver := func(e LogEntry) {
		mu.Lock()
		defer mu.Unlock()
		if !backfilled {
			buffered = append(buffered, e)
			return
		}
		if e.ID <= *lastSeen {
			return
		}
		*lastSeen = e.ID
		emit(e)
	}

	scanDone := make(chan error, 1)
	go func() {
		scanDone <- scanEvents(resp.Body, deliver)
	}()

	if err := s.backfill(ctx, lastSeen, emit, &mu, &buffered, &backfilled); err != nil {
		// A failed backfill is not fatal: the live stream is still useful, and
		// the gap is reported rather than hidden.
		s.log.Warn("log backfill failed; the feed may have a gap", "error", err)
		mu.Lock()
		backfilled = true
		for _, e := range buffered {
			if e.ID > *lastSeen {
				*lastSeen = e.ID
				emit(e)
			}
		}
		buffered = nil
		mu.Unlock()
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-scanDone:
		return err
	}
}

// backfill fetches everything written since lastSeen, then releases whatever
// the live stream buffered while it was doing so.
func (s *LogStreamer) backfill(
	ctx context.Context,
	lastSeen *int,
	emit func(LogEntry),
	mu *sync.Mutex,
	buffered *[]LogEntry,
	backfilled *bool,
) error {
	// fetch pulls either recent history (cold start) or the gap since cursor.
	fetch := func(cursor int) (LogPage, error) {
		if cursor < 0 {
			return s.client.Log(ctx, -1, -seedLines)
		}
		return s.client.Log(ctx, cursor+1, backfillLines)
	}

	page, err := fetch(*lastSeen)
	if err != nil {
		return err
	}

	// The server's log ids restart from zero when the game server restarts. A
	// log shorter than what we have already seen is the observable signal.
	// Without this every later line looks like a duplicate and the feed goes
	// permanently silent.
	//
	// Resetting the cursor is not enough on its own: the gap query asked for
	// lines after a cursor the server no longer has, so it came back empty and
	// there is nothing to emit. Re-seed from the restarted log instead.
	if *lastSeen >= 0 && page.LastLine < *lastSeen {
		s.log.Info("game server log restarted; re-seeding from the new log",
			"previousCursor", *lastSeen, "serverLastLine", page.LastLine)
		*lastSeen = -1
		if page, err = fetch(-1); err != nil {
			return err
		}
	}

	mu.Lock()
	defer mu.Unlock()

	for _, e := range page.Entries {
		if e.ID > *lastSeen {
			*lastSeen = e.ID
			emit(e)
		}
	}

	*backfilled = true
	for _, e := range *buffered {
		if e.ID > *lastSeen {
			*lastSeen = e.ID
			emit(e)
		}
	}
	*buffered = nil
	return nil
}

func (s *LogStreamer) open(ctx context.Context) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.client.baseURL+ssePath, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set(HeaderTokenName, s.client.tokenName)
	req.Header.Set(HeaderTokenSecret, s.client.tokenSecret)
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")

	resp, err := s.client.streamHTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("open log stream: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, &APIError{Status: resp.StatusCode, RequestSubpath: ssePath}
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("log stream returned content-type %q, want text/event-stream", ct)
	}
	return resp, nil
}

// scanEvents parses the SSE framing and calls emit for each logLine event.
//
// Returns nil when the server closes the stream cleanly.
func scanEvents(r io.Reader, emit func(LogEntry)) error {
	scanner := bufio.NewScanner(r)
	// Log lines can carry long stack traces, so the default 64 KiB token limit
	// is not generous enough.
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)

	var (
		eventName string
		data      strings.Builder
	)

	dispatch := func() {
		defer func() {
			eventName = ""
			data.Reset()
		}()
		if eventName != sseEventName || data.Len() == 0 {
			return
		}
		var entry LogEntry
		if err := json.Unmarshal([]byte(data.String()), &entry); err != nil {
			// One malformed frame should not tear down an otherwise healthy
			// stream.
			return
		}
		emit(entry)
	}

	for scanner.Scan() {
		line := scanner.Text()

		switch {
		case line == "":
			dispatch()
		case strings.HasPrefix(line, ":"):
			// Comment or keep-alive.
		case strings.HasPrefix(line, "event:"):
			eventName = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			if data.Len() > 0 {
				data.WriteByte('\n')
			}
			data.WriteString(strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		default:
			// id:, retry: and any unknown field are not used here.
		}
	}

	err := scanner.Err()
	if err == nil || errors.Is(err, io.EOF) {
		return nil
	}
	return err
}

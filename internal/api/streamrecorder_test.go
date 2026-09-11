package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// streamRecorder is an httptest.ResponseRecorder that is safe to read while the
// handler is still writing, which a streaming handler always is.
type streamRecorder struct {
	*httptest.ResponseRecorder
	mu   sync.Mutex
	buf  bytes.Buffer
	sync chan struct{}
}

func newStreamRecorder() *streamRecorder {
	return &streamRecorder{
		ResponseRecorder: httptest.NewRecorder(),
		sync:             make(chan struct{}, 64),
	}
}

func (r *streamRecorder) Write(b []byte) (int, error) {
	r.mu.Lock()
	r.buf.Write(b)
	r.mu.Unlock()
	select {
	case r.sync <- struct{}{}:
	default:
	}
	return len(b), nil
}

func (r *streamRecorder) Header() http.Header { return r.ResponseRecorder.Header() }

func (r *streamRecorder) WriteHeader(code int) { r.ResponseRecorder.WriteHeader(code) }

// Flush satisfies http.Flusher so http.ResponseController can drive it.
func (r *streamRecorder) Flush() {}

func (r *streamRecorder) body() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.buf.String()
}

func (r *streamRecorder) waitFor(t *testing.T, substr string, within time.Duration) {
	t.Helper()
	deadline := time.After(within)
	for {
		if strings.Contains(r.body(), substr) {
			return
		}
		select {
		case <-r.sync:
		case <-deadline:
			t.Fatalf("timed out waiting for %q in stream; got:\n%s", substr, r.body())
		}
	}
}

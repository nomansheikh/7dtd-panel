package httpx

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func quiet() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestRecoverTurnsPanicIntoFiveHundred(t *testing.T) {
	// A bug in one handler must not take the whole panel down.
	h := Recover(quiet())(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	// The panic value must not reach the client.
	if strings.Contains(rec.Body.String(), "boom") {
		t.Error("the response leaks the panic value")
	}
}

func TestRecoverRepanicsOnAbortHandler(t *testing.T) {
	// ErrAbortHandler is how a client disconnect surfaces; swallowing it would
	// log a spurious error and write to a dead connection.
	h := Recover(quiet())(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic(http.ErrAbortHandler)
	}))

	defer func() {
		if rec := recover(); rec != http.ErrAbortHandler {
			t.Errorf("recovered %v, want ErrAbortHandler to propagate", rec)
		}
	}()
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
}

func TestRequestIDIsGeneratedAndEchoed(t *testing.T) {
	var seen string
	h := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = RequestIDFrom(r.Context())
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if seen == "" {
		t.Error("no request id was attached to the context")
	}
	if got := rec.Header().Get("X-Request-Id"); got != seen {
		t.Errorf("header %q does not match context value %q", got, seen)
	}
}

func TestRequestIDHonoursAndBoundsIncomingValue(t *testing.T) {
	t.Run("incoming id is reused for correlation", func(t *testing.T) {
		var seen string
		h := RequestID(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			seen = RequestIDFrom(r.Context())
		}))
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Request-Id", "upstream-id")
		h.ServeHTTP(httptest.NewRecorder(), req)

		if seen != "upstream-id" {
			t.Errorf("id = %q, want upstream-id", seen)
		}
	})

	t.Run("absurdly long id is truncated", func(t *testing.T) {
		var seen string
		h := RequestID(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			seen = RequestIDFrom(r.Context())
		}))
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Request-Id", strings.Repeat("x", 500))
		h.ServeHTTP(httptest.NewRecorder(), req)

		if len(seen) > 64 {
			t.Errorf("id is %d chars, want at most 64", len(seen))
		}
	})
}

func TestSecurityHeaders(t *testing.T) {
	h := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	want := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "no-referrer",
	}
	for k, v := range want {
		if got := rec.Header().Get(k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}

	csp := rec.Header().Get("Content-Security-Policy")
	// connect-src 'self' is what keeps a compromised dependency from shipping
	// the operator's session elsewhere.
	for _, directive := range []string{
		"default-src 'self'", "connect-src 'self'", "object-src 'none'",
		"frame-ancestors 'none'", "base-uri 'none'",
	} {
		if !strings.Contains(csp, directive) {
			t.Errorf("CSP is missing %q: %s", directive, csp)
		}
	}
}

func TestClientIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		headers    map[string]string
		trustProxy bool
		want       string
	}{
		{
			name:       "direct connection",
			remoteAddr: "192.0.2.10:54321",
			want:       "192.0.2.10",
		},
		{
			// Believing this unconditionally would let any client forge the
			// address recorded in the logs and the rate limiter's key.
			name:       "forwarded header ignored when proxy is untrusted",
			remoteAddr: "192.0.2.10:54321",
			headers:    map[string]string{"X-Forwarded-For": "203.0.113.5"},
			want:       "192.0.2.10",
		},
		{
			name:       "forwarded header used when proxy is trusted",
			remoteAddr: "10.0.0.1:443",
			headers:    map[string]string{"X-Forwarded-For": "203.0.113.5"},
			trustProxy: true,
			want:       "203.0.113.5",
		},
		{
			name:       "first hop wins in a forwarded chain",
			remoteAddr: "10.0.0.1:443",
			headers:    map[string]string{"X-Forwarded-For": "203.0.113.5, 10.0.0.9"},
			trustProxy: true,
			want:       "203.0.113.5",
		},
		{
			name:       "x-real-ip is a fallback",
			remoteAddr: "10.0.0.1:443",
			headers:    map[string]string{"X-Real-Ip": "203.0.113.7"},
			trustProxy: true,
			want:       "203.0.113.7",
		},
		{
			name:       "ipv6 remote address",
			remoteAddr: "[2001:db8::1]:443",
			want:       "2001:db8::1",
		},
		{
			name:       "unparseable remote address is returned as-is",
			remoteAddr: "not-an-address",
			want:       "not-an-address",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			if got := ClientIP(req, tt.trustProxy); got != tt.want {
				t.Errorf("ClientIP = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestChainOrdersOutermostFirst(t *testing.T) {
	var order []string
	mark := func(name string) Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, name)
				next.ServeHTTP(w, r)
			})
		}
	}
	h := Chain(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		order = append(order, "handler")
	}), mark("first"), mark("second"))

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	want := []string{"first", "second", "handler"}
	if len(order) != len(want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Errorf("order = %v, want %v", order, want)
			break
		}
	}
}

func TestDecodeJSONRejectsBadInput(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
	}
	tests := []struct {
		name        string
		body        string
		contentType string
		wantErr     bool
	}{
		{name: "valid", body: `{"name":"x"}`, contentType: "application/json"},
		{name: "charset suffix is fine", body: `{"name":"x"}`, contentType: "application/json; charset=utf-8"},
		{name: "no content type is allowed", body: `{"name":"x"}`},
		{name: "wrong content type", body: `{"name":"x"}`, contentType: "text/plain", wantErr: true},
		{name: "empty body", body: "", contentType: "application/json", wantErr: true},
		{name: "malformed", body: `{"name":`, contentType: "application/json", wantErr: true},
		// A misspelled field silently ignored is how a caller ends up debugging
		// why their value had no effect.
		{name: "unknown field", body: `{"nme":"x"}`, contentType: "application/json", wantErr: true},
		{name: "two documents", body: `{"name":"a"}{"name":"b"}`, contentType: "application/json", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tt.body))
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			var p payload
			err := DecodeJSON(httptest.NewRecorder(), req, &p)
			if tt.wantErr && err == nil {
				t.Error("DecodeJSON succeeded, want error")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("DecodeJSON: %v", err)
			}
		})
	}
}

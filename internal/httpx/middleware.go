// Package httpx holds HTTP plumbing shared by the panel's handlers.
package httpx

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"time"
)

type ctxKey int

const (
	ctxKeyRequestID ctxKey = iota
	ctxKeyLogger
)

// Middleware is a handler decorator.
type Middleware func(http.Handler) http.Handler

// Chain applies middleware so that the first listed runs outermost.
func Chain(h http.Handler, mw ...Middleware) http.Handler {
	for i := len(mw) - 1; i >= 0; i-- {
		h = mw[i](h)
	}
	return h
}

// statusRecorder captures the status code for logging.
type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *statusRecorder) WriteHeader(code int) {
	if r.status == 0 {
		r.status = code
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

// Unwrap lets http.ResponseController reach the underlying writer, which the
// SSE handler needs in order to flush.
func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

// RequestID attaches a short request identifier for log correlation.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-Id")
		if id == "" {
			id = newID()
		} else if len(id) > 64 {
			id = id[:64]
		}
		ctx := context.WithValue(r.Context(), ctxKeyRequestID, id)
		w.Header().Set("X-Request-Id", id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequestIDFrom returns the request ID, or "" if unset.
func RequestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(ctxKeyRequestID).(string)
	return id
}

// Logger logs one line per request at debug, raising to warn or error for
// failures so normal traffic stays quiet.
func Logger(log *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w}

			reqLog := log.With("requestId", RequestIDFrom(r.Context()))
			ctx := context.WithValue(r.Context(), ctxKeyLogger, reqLog)

			next.ServeHTTP(rec, r.WithContext(ctx))

			status := rec.status
			if status == 0 {
				status = http.StatusOK
			}
			attrs := []any{
				"method", r.Method,
				"path", r.URL.Path,
				"status", status,
				"duration", time.Since(start).Round(time.Millisecond).String(),
				"remoteIp", ClientIP(r, false),
			}
			switch {
			case status >= 500:
				reqLog.Error("request failed", attrs...)
			case status >= 400:
				reqLog.Warn("request rejected", attrs...)
			default:
				reqLog.Debug("request", attrs...)
			}
		})
	}
}

// LoggerFrom returns the request-scoped logger, falling back to the default.
func LoggerFrom(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(ctxKeyLogger).(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}

// Recover turns a panic in a handler into a 500 instead of taking the process
// down. A bug in one endpoint must not make the whole panel unavailable.
func Recover(log *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				rec := recover()
				if rec == nil {
					return
				}
				// A client disconnecting mid-write surfaces as this panic and
				// is not a bug worth an error log.
				if rec == http.ErrAbortHandler {
					panic(rec)
				}
				log.Error("handler panicked",
					"requestId", RequestIDFrom(r.Context()),
					"method", r.Method,
					"path", r.URL.Path,
					"panic", rec,
					"stack", string(debug.Stack()))
				http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// SecurityHeaders sets defensive response headers.
//
// The CSP is deliberately strict: the panel serves its own bundle and talks
// only to its own origin, so there is no reason to allow anything else. That
// also means a compromised dependency cannot exfiltrate the operator's session.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Content-Security-Policy", strings.Join([]string{
			"default-src 'self'",
			// Vite injects styles at runtime, so inline styles are required.
			"style-src 'self' 'unsafe-inline'",
			"img-src 'self' data: blob:",
			"connect-src 'self'",
			"font-src 'self' data:",
			"object-src 'none'",
			"base-uri 'none'",
			"frame-ancestors 'none'",
			"form-action 'self'",
		}, "; "))
		next.ServeHTTP(w, r)
	})
}

// ClientIP extracts the client address. X-Forwarded-For is only consulted when
// the operator has declared that the panel sits behind a trusted proxy;
// otherwise any client could spoof its own address into the logs.
func ClientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			if first, _, ok := strings.Cut(xff, ","); ok {
				return strings.TrimSpace(first)
			}
			return strings.TrimSpace(xff)
		}
		if xrip := r.Header.Get("X-Real-Ip"); xrip != "" {
			return strings.TrimSpace(xrip)
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

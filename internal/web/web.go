// Package web serves the built frontend from the binary.
//
// One process serves both the API and the UI: one container, no sidecar, no
// supervisor. Vite is configured to build into this package's dist directory so
// go:embed can reach it, since embed patterns cannot traverse upwards.
package web

import (
	"embed"
	"errors"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// all: includes dotfiles, so the committed dist/.gitkeep satisfies the embed
// pattern when the frontend has not been built. That keeps "go build ./..."
// working on a clean checkout without a Node toolchain.
//
//go:embed all:dist
var embedded embed.FS

// ErrNotBuilt means no frontend build is embedded in this binary.
var ErrNotBuilt = errors.New("web: frontend was not built into this binary")

// FS returns the embedded build rooted at dist.
func FS() (fs.FS, error) {
	sub, err := fs.Sub(embedded, "dist")
	if err != nil {
		return nil, err
	}
	if _, err := fs.Stat(sub, "index.html"); err != nil {
		return sub, ErrNotBuilt
	}
	return sub, nil
}

// Handler serves the frontend with SPA fallback: unknown paths return
// index.html so client-side routes survive a page reload.
//
// When no build is embedded it serves a plain explanation rather than a blank
// 404, because the most likely cause is someone building the Go binary without
// running the frontend build.
func Handler() http.Handler {
	sub, err := FS()
	if errors.Is(err, ErrNotBuilt) {
		return http.HandlerFunc(notBuilt)
	}
	if err != nil {
		return http.HandlerFunc(notBuilt)
	}

	fileServer := http.FileServerFS(sub)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cleaned := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if cleaned == "" {
			cleaned = "index.html"
		}

		if info, err := fs.Stat(sub, cleaned); err == nil && !info.IsDir() {
			// Vite emits content-hashed asset filenames, so those are safe to
			// cache forever. index.html must never be, or an upgrade would
			// keep serving the old bundle.
			if strings.HasPrefix(cleaned, "assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			} else {
				w.Header().Set("Cache-Control", "no-cache")
			}
			fileServer.ServeHTTP(w, r)
			return
		}

		// A missing asset is a genuine 404; anything else is a client route.
		if strings.HasPrefix(cleaned, "assets/") {
			http.NotFound(w, r)
			return
		}
		serveIndex(w, r, sub)
	})
}

func serveIndex(w http.ResponseWriter, r *http.Request, sub fs.FS) {
	body, err := fs.ReadFile(sub, "index.html")
	if err != nil {
		notBuilt(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(body)
}

func notBuilt(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusServiceUnavailable)
	_, _ = w.Write([]byte("The panel UI was not built into this binary.\n\n" +
		"The API is still available under /api. To build the UI, run:\n" +
		"    make frontend\n"))
}

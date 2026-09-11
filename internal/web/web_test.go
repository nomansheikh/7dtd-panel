package web

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// These tests run against whatever is embedded. On a clean checkout that is
// only dist/.gitkeep, so the not-built path is what gets exercised in CI; after
// a frontend build the served-file path is. Both are asserted conditionally so
// the suite is honest either way rather than passing vacuously.

func TestFSReportsWhetherABuildIsEmbedded(t *testing.T) {
	_, err := FS()
	if err != nil && !errors.Is(err, ErrNotBuilt) {
		t.Fatalf("FS returned an unexpected error: %v", err)
	}
	if errors.Is(err, ErrNotBuilt) {
		t.Log("no frontend build embedded; exercising the not-built path")
	} else {
		t.Log("frontend build is embedded; exercising the served-file path")
	}
}

func TestHandlerNeverPanics(t *testing.T) {
	h := Handler()
	// Includes traversal attempts and odd paths, which must be handled rather
	// than reaching the filesystem or crashing.
	paths := []string{
		"/", "/index.html", "/login", "/players/123",
		"/assets/app.js", "/assets/../../etc/passwd",
		"/../../etc/passwd", "//", "/%2e%2e/", "/a/b/c/d/e",
	}
	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
			if rec.Code == 0 {
				t.Error("handler wrote no status")
			}
		})
	}
}

func TestNotBuiltExplainsItself(t *testing.T) {
	_, err := FS()
	if !errors.Is(err, ErrNotBuilt) {
		t.Skip("a frontend build is embedded; nothing to assert here")
	}

	rec := httptest.NewRecorder()
	Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
	// A blank 404 would send someone hunting for a routing bug when the real
	// cause is a skipped frontend build.
	body := rec.Body.String()
	for _, want := range []string{"not built", "make frontend", "/api"} {
		if !strings.Contains(body, want) {
			t.Errorf("body does not mention %q:\n%s", want, body)
		}
	}
}

func TestAssetCachingHeaders(t *testing.T) {
	sub, err := FS()
	if errors.Is(err, ErrNotBuilt) {
		t.Skip("no frontend build embedded")
	}
	if err != nil {
		t.Fatalf("FS: %v", err)
	}
	_ = sub

	h := Handler()

	t.Run("index.html is never cached", func(t *testing.T) {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		// Caching index.html would keep serving the old bundle after an upgrade.
		if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
			t.Errorf("Cache-Control = %q, want no-cache", got)
		}
	})
}

func TestUnknownRouteFallsBackToIndex(t *testing.T) {
	if _, err := FS(); errors.Is(err, ErrNotBuilt) {
		t.Skip("no frontend build embedded")
	}
	rec := httptest.NewRecorder()
	// A client-side route must survive a page reload.
	Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/dashboard", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 so the SPA can route", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Errorf("Content-Type = %q, want html", ct)
	}
}

func TestMissingAssetIsA404(t *testing.T) {
	if _, err := FS(); errors.Is(err, ErrNotBuilt) {
		t.Skip("no frontend build embedded")
	}
	rec := httptest.NewRecorder()
	// Falling back to index.html for a missing asset would make a broken script
	// tag silently return HTML.
	Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/assets/missing.js", nil))

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

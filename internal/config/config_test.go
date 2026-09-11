package config

import (
	"strings"
	"testing"
	"time"
)

// env builds a getenv function from a map, so tests never touch the real
// process environment and can run in parallel.
func env(vars map[string]string) func(string) string {
	return func(k string) string { return vars[k] }
}

// minimal is the smallest environment that validates.
func minimal() map[string]string {
	return map[string]string{
		"SDTD_HOST":             "10.0.0.5",
		"SDTD_API_TOKEN_NAME":   "panel",
		"SDTD_API_TOKEN_SECRET": "a-secret-value",
		"PANEL_ADMIN_PASSWORD":  "correct-horse",
	}
}

func TestLoadAppliesDefaults(t *testing.T) {
	c, err := Load(env(minimal()))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	tests := []struct {
		name string
		got  any
		want any
	}{
		{"game port", c.Game.Port, 8080},
		{"game scheme", c.Game.Scheme, "http"},
		{"panel port", c.Panel.Port, 8080},
		{"admin username", c.Panel.AdminUsername, "admin"},
		{"db path", c.Panel.DBPath, "/data/panel.db"},
		{"session ttl", c.Panel.SessionTTL, 168 * time.Hour},
		{"trust proxy", c.Panel.TrustProxy, false},
		{"poll interval", c.Panel.PollInterval, 5 * time.Second},
		{"failure threshold", c.Panel.FailureThreshold, 3},
		{"allow destructive", c.Panel.AllowDestructive, true},
		{"log level", c.Panel.LogLevel, "info"},
		{"log format", c.Panel.LogFormat, "json"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %v, want %v", tt.got, tt.want)
			}
		})
	}
}

func TestBaseURL(t *testing.T) {
	tests := []struct {
		name string
		host string
		port string
		sch  string
		want string
	}{
		{"default http", "10.0.0.5", "", "", "http://10.0.0.5:8080"},
		{"custom port", "game.lan", "8100", "", "http://game.lan:8100"},
		{"https", "game.example.com", "443", "https", "https://game.example.com:443"},
		// A bare IPv6 literal has to come out bracketed or the URL is invalid.
		{"ipv6 is bracketed", "fd00::1", "8080", "", "http://[fd00::1]:8080"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vars := minimal()
			vars["SDTD_HOST"] = tt.host
			if tt.port != "" {
				vars["SDTD_API_PORT"] = tt.port
			}
			if tt.sch != "" {
				vars["SDTD_API_SCHEME"] = tt.sch
			}
			c, err := Load(env(vars))
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if got := c.Game.BaseURL(); got != tt.want {
				t.Errorf("BaseURL = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLoadRejectsBadValues(t *testing.T) {
	tests := []struct {
		name     string
		override map[string]string
		wantIn   string
	}{
		{"missing host", map[string]string{"SDTD_HOST": ""}, "SDTD_HOST is required"},
		{"missing token name", map[string]string{"SDTD_API_TOKEN_NAME": ""}, "SDTD_API_TOKEN_NAME is required"},
		{"missing token secret", map[string]string{"SDTD_API_TOKEN_SECRET": ""}, "SDTD_API_TOKEN_SECRET is required"},
		{"missing admin password", map[string]string{"PANEL_ADMIN_PASSWORD": ""}, "PANEL_ADMIN_PASSWORD is required"},
		{"short admin password", map[string]string{"PANEL_ADMIN_PASSWORD": "short"}, "at least 8"},
		{"non-numeric port", map[string]string{"SDTD_API_PORT": "eighty"}, "must be a number"},
		{"port out of range", map[string]string{"SDTD_API_PORT": "70000"}, "between 1 and 65535"},
		{"bad scheme", map[string]string{"SDTD_API_SCHEME": "ftp"}, "must be one of http, https"},
		{"bad duration", map[string]string{"PANEL_SESSION_TTL": "forever"}, "must be a duration"},
		{"negative duration", map[string]string{"PANEL_SESSION_TTL": "-1h"}, "must be positive"},
		{"poll interval too small", map[string]string{"PANEL_POLL_INTERVAL": "100ms"}, "at least 1s"},
		{"bad bool", map[string]string{"PANEL_TRUST_PROXY": "yes please"}, "must be true or false"},
		{"threshold below one", map[string]string{"PANEL_FAILURE_THRESHOLD": "0"}, "at least 1"},
		{"bad log level", map[string]string{"PANEL_LOG_LEVEL": "verbose"}, "must be one of debug, info, warn, error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vars := minimal()
			for k, v := range tt.override {
				vars[k] = v
			}
			_, err := Load(env(vars))
			if err == nil {
				t.Fatal("Load succeeded, want error")
			}
			if !strings.Contains(err.Error(), tt.wantIn) {
				t.Errorf("error %q does not mention %q", err, tt.wantIn)
			}
		})
	}
}

func TestLoadReportsEveryProblemAtOnce(t *testing.T) {
	// Failing one variable at a time turns first-run setup into a guessing
	// game, so all problems must surface together.
	_, err := Load(env(map[string]string{"SDTD_API_PORT": "nope"}))
	if err == nil {
		t.Fatal("Load succeeded, want error")
	}
	for _, want := range []string{
		"SDTD_HOST is required",
		"SDTD_API_TOKEN_NAME is required",
		"SDTD_API_TOKEN_SECRET is required",
		"PANEL_ADMIN_PASSWORD is required",
		"must be a number",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error is missing %q:\n%v", want, err)
		}
	}
}

func TestValuesAreTrimmedAndSchemeIsCaseInsensitive(t *testing.T) {
	vars := minimal()
	vars["SDTD_HOST"] = "  10.0.0.9  "
	vars["SDTD_API_SCHEME"] = "HTTPS"
	c, err := Load(env(vars))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Game.Host != "10.0.0.9" {
		t.Errorf("host = %q, want trimmed", c.Game.Host)
	}
	if c.Game.Scheme != "https" {
		t.Errorf("scheme = %q, want https", c.Game.Scheme)
	}
}

func TestRedactedOmitsSecrets(t *testing.T) {
	vars := minimal()
	vars["SDTD_API_TOKEN_SECRET"] = "super-secret-token"
	vars["PANEL_ADMIN_PASSWORD"] = "super-secret-password"
	c, err := Load(env(vars))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Redacted feeds the startup log, which frequently ends up in bug reports.
	rendered := ""
	for k, v := range c.Redacted() {
		rendered += k + "=" + toString(v) + ";"
	}
	for _, secret := range []string{"super-secret-token", "super-secret-password"} {
		if strings.Contains(rendered, secret) {
			t.Errorf("Redacted leaked %q", secret)
		}
	}
	if !strings.Contains(rendered, "panel") {
		t.Error("Redacted should still include the token name, which is not secret")
	}
}

func toString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	default:
		return "non-string"
	}
}

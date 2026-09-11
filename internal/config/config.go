// Package config loads the panel's configuration from environment variables.
//
// Environment variables are the only source. There is deliberately no config
// file: needing to mount one is the single most common annoyance in
// self-hosted software, and every optional setting here has a usable default.
package config

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the fully validated configuration. A zero Config is never valid;
// always obtain one from Load.
type Config struct {
	// Servers is never empty after a successful Load.
	Servers []Game
	Panel   Panel
}

// Game describes how to reach one 7 Days to Die server. The panel and the game
// servers are assumed to be on different hosts and to share no filesystem.
type Game struct {
	// ID is the stable identifier used in URLs. Lower case, alphanumeric and
	// dashes.
	ID string
	// Name is what an operator sees. Defaults to the ID.
	Name        string
	Host        string
	Port        int
	Scheme      string
	TokenName   string
	TokenSecret string
}

// Default returns the first configured server, which is the one the UI opens
// on. Load guarantees at least one.
func (c Config) Default() Game { return c.Servers[0] }

// Server finds a server by ID.
func (c Config) Server(id string) (Game, bool) {
	for _, g := range c.Servers {
		if g.ID == id {
			return g, true
		}
	}
	return Game{}, false
}

// BaseURL is the root of the game server's web API, without a trailing slash.
func (g Game) BaseURL() string {
	return fmt.Sprintf("%s://%s", g.Scheme, net.JoinHostPort(g.Host, strconv.Itoa(g.Port)))
}

// Panel describes the panel's own behaviour.
type Panel struct {
	Port             int
	AdminUsername    string
	AdminPassword    string
	DBPath           string
	SessionTTL       time.Duration
	TrustProxy       bool
	PollInterval     time.Duration
	FailureThreshold int
	AllowDestructive bool
	LogLevel         string
	LogFormat        string
}

// Load reads and validates configuration from the environment.
//
// Every problem is collected and reported together. Failing on one variable
// at a time turns first-run setup into a guessing game.
func Load(getenv func(string) string) (Config, error) {
	if getenv == nil {
		getenv = os.Getenv
	}
	p := &parser{getenv: getenv}

	var c Config

	c.Servers = p.servers()

	c.Panel.Port = p.port("PANEL_PORT", 8080)
	c.Panel.AdminUsername = p.str("PANEL_ADMIN_USERNAME", "admin")
	c.Panel.AdminPassword = p.required("PANEL_ADMIN_PASSWORD")
	c.Panel.DBPath = p.str("PANEL_DB_PATH", "/data/panel.db")
	c.Panel.SessionTTL = p.duration("PANEL_SESSION_TTL", 168*time.Hour)
	c.Panel.TrustProxy = p.boolean("PANEL_TRUST_PROXY", false)
	c.Panel.PollInterval = p.duration("PANEL_POLL_INTERVAL", 5*time.Second)
	c.Panel.FailureThreshold = p.intAtLeast("PANEL_FAILURE_THRESHOLD", 3, 1)
	c.Panel.AllowDestructive = p.boolean("PANEL_ALLOW_DESTRUCTIVE", true)
	c.Panel.LogLevel = p.enum("PANEL_LOG_LEVEL", "info", "debug", "info", "warn", "error")
	c.Panel.LogFormat = p.enum("PANEL_LOG_FORMAT", "json", "json", "text")

	// A password short enough to brute-force defeats the point of hashing it.
	if n := len(c.Panel.AdminPassword); n > 0 && n < 8 {
		p.errf("PANEL_ADMIN_PASSWORD is %d characters; use at least 8", n)
	}
	if c.Panel.PollInterval < time.Second {
		p.errf("PANEL_POLL_INTERVAL is %s; use at least 1s to avoid hammering the game server", c.Panel.PollInterval)
	}

	if err := p.err(); err != nil {
		return Config{}, err
	}
	return c, nil
}

// parser accumulates problems so Load can report all of them at once.
type parser struct {
	getenv   func(string) string
	problems []string
}

func (p *parser) errf(format string, args ...any) {
	p.problems = append(p.problems, fmt.Sprintf(format, args...))
}

func (p *parser) err() error {
	if len(p.problems) == 0 {
		return nil
	}
	return errors.New("invalid configuration:\n  - " + strings.Join(p.problems, "\n  - "))
}

func (p *parser) required(key string) string {
	v := strings.TrimSpace(p.getenv(key))
	if v == "" {
		p.errf("%s is required", key)
	}
	return v
}

func (p *parser) str(key, def string) string {
	if v := strings.TrimSpace(p.getenv(key)); v != "" {
		return v
	}
	return def
}

func (p *parser) port(key string, def int) int {
	raw := strings.TrimSpace(p.getenv(key))
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		p.errf("%s must be a number, got %q", key, raw)
		return def
	}
	if n < 1 || n > 65535 {
		p.errf("%s must be between 1 and 65535, got %d", key, n)
		return def
	}
	return n
}

func (p *parser) intAtLeast(key string, def, min int) int {
	raw := strings.TrimSpace(p.getenv(key))
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		p.errf("%s must be a number, got %q", key, raw)
		return def
	}
	if n < min {
		p.errf("%s must be at least %d, got %d", key, min, n)
		return def
	}
	return n
}

func (p *parser) duration(key string, def time.Duration) time.Duration {
	raw := strings.TrimSpace(p.getenv(key))
	if raw == "" {
		return def
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		p.errf("%s must be a duration such as 30s or 168h, got %q", key, raw)
		return def
	}
	if d <= 0 {
		p.errf("%s must be positive, got %s", key, d)
		return def
	}
	return d
}

func (p *parser) boolean(key string, def bool) bool {
	raw := strings.TrimSpace(p.getenv(key))
	if raw == "" {
		return def
	}
	b, err := strconv.ParseBool(raw)
	if err != nil {
		p.errf("%s must be true or false, got %q", key, raw)
		return def
	}
	return b
}

func (p *parser) enum(key, def string, allowed ...string) string {
	raw := strings.ToLower(strings.TrimSpace(p.getenv(key)))
	if raw == "" {
		return def
	}
	for _, a := range allowed {
		if raw == a {
			return raw
		}
	}
	p.errf("%s must be one of %s, got %q", key, strings.Join(allowed, ", "), raw)
	return def
}

// Redacted returns the config with secrets removed, for logging at startup.
func (c Config) Redacted() map[string]any {
	servers := make([]string, 0, len(c.Servers))
	for _, g := range c.Servers {
		// The token name is not secret; the secret is never included.
		servers = append(servers, g.ID+"="+g.BaseURL()+" (token "+g.TokenName+")")
	}
	return map[string]any{
		"game.servers":           strings.Join(servers, ", "),
		"game.count":             len(c.Servers),
		"panel.port":             c.Panel.Port,
		"panel.adminUsername":    c.Panel.AdminUsername,
		"panel.dbPath":           c.Panel.DBPath,
		"panel.sessionTTL":       c.Panel.SessionTTL.String(),
		"panel.trustProxy":       c.Panel.TrustProxy,
		"panel.pollInterval":     c.Panel.PollInterval.String(),
		"panel.failureThreshold": c.Panel.FailureThreshold,
		"panel.allowDestructive": c.Panel.AllowDestructive,
		"panel.logLevel":         c.Panel.LogLevel,
	}
}

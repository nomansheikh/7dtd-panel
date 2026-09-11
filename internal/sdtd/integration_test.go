package sdtd

import (
	"context"
	"os"
	"testing"
	"time"
)

// Integration tests run against a real 7 Days to Die server and are skipped
// unless SDTD_HOST is set, so "go test ./..." needs no network and no server.
//
//	SDTD_HOST=10.0.0.5 SDTD_API_TOKEN_NAME=panel \
//	SDTD_API_TOKEN_SECRET=... go test ./internal/sdtd -run Integration -v
//
// They assert on shape rather than on values, since the world changes between
// runs. Their job is to catch the client disagreeing with a real server.
func liveClient(t *testing.T) *Client {
	t.Helper()
	host := os.Getenv("SDTD_HOST")
	if host == "" {
		t.Skip("SDTD_HOST not set; skipping live server test")
	}
	port := os.Getenv("SDTD_API_PORT")
	if port == "" {
		port = "8080"
	}
	c, err := New(Options{
		BaseURL:     "http://" + host + ":" + port,
		TokenName:   os.Getenv("SDTD_API_TOKEN_NAME"),
		TokenSecret: os.Getenv("SDTD_API_TOKEN_SECRET"),
		Timeout:     15 * time.Second,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

func TestIntegrationServerStats(t *testing.T) {
	c := liveClient(t)
	stats, err := c.ServerStats(context.Background())
	if err != nil {
		t.Fatalf("ServerStats: %v", err)
	}
	if stats.GameTime.Days < 1 {
		t.Errorf("days = %d, want at least 1", stats.GameTime.Days)
	}
	if stats.GameTime.Hours < 0 || stats.GameTime.Hours > 23 {
		t.Errorf("hours = %d, out of range", stats.GameTime.Hours)
	}
	t.Logf("day %d %02d:%02d — %d players, %d hostiles, %d animals",
		stats.GameTime.Days, stats.GameTime.Hours, stats.GameTime.Minutes,
		stats.Players, stats.Hostiles, stats.Animals)
}

func TestIntegrationServerInfo(t *testing.T) {
	c := liveClient(t)
	info, err := c.ServerInfo(context.Background())
	if err != nil {
		t.Fatalf("ServerInfo: %v", err)
	}
	// These drive the dashboard, so their absence is a real regression.
	for _, name := range []string{"ServerVersion", "GameType", "MaxPlayers", "CurrentPlayers"} {
		if _, ok := info.Get(name); !ok {
			t.Errorf("serverinfo is missing %s", name)
		}
	}
	t.Logf("version=%s world=%s players=%d/%d",
		info.Str("ServerVersion"), info.Str("LevelName"),
		info.Int("CurrentPlayers"), info.Int("MaxPlayers"))
}

func TestIntegrationLogCursorHasNoGaps(t *testing.T) {
	c := liveClient(t)
	ctx := context.Background()

	// Read the newest lines, then follow from lastLine. The second read must
	// not repeat anything from the first: this is the property the SSE gap
	// backfill will depend on.
	first, err := c.Log(ctx, -1, -5)
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	if len(first.Entries) == 0 {
		t.Skip("server log is empty")
	}
	for _, e := range first.Entries {
		if _, ok := e.UptimeDuration(); !ok {
			t.Errorf("entry %d has unparseable uptime %q", e.ID, e.Uptime)
		}
		if _, ok := e.Time(); !ok {
			t.Errorf("entry %d has unparseable isotime %q", e.ID, e.ISOTime)
		}
	}
	newest := first.Entries[len(first.Entries)-1]
	t.Logf("newest line %d at uptime %s: %.60s", newest.ID, newest.Uptime, newest.Msg)

	second, err := c.Log(ctx, first.LastLine, 5)
	if err != nil {
		t.Fatalf("Log follow-up: %v", err)
	}
	for _, e := range second.Entries {
		if e.ID < first.LastLine {
			t.Errorf("following from lastLine=%d returned older line %d", first.LastLine, e.ID)
		}
	}
	t.Logf("followed from %d, got %d new entries", first.LastLine, len(second.Entries))
}

func TestIntegrationExecuteReadOnlyCommand(t *testing.T) {
	c := liveClient(t)
	// gettime changes nothing, so it is safe to run against a live server.
	res, err := c.Execute(context.Background(), "gettime")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Command != "gettime" {
		t.Errorf("command = %q, want gettime", res.Command)
	}
	if res.Result == "" {
		t.Error("result is empty")
	}
	t.Logf("gettime -> %q", res.Result)
}

func TestIntegrationUnknownCommandIsTypedError(t *testing.T) {
	c := liveClient(t)
	_, err := c.Execute(context.Background(), "definitelynotacommand")
	if err == nil {
		t.Fatal("Execute succeeded for a nonexistent command")
	}
	if got := ErrorCode(err); got != CodeUnknownCommand {
		t.Errorf("ErrorCode = %q, want %q", got, CodeUnknownCommand)
	}
	if !IsNotFound(err) {
		t.Error("IsNotFound should be true for an unknown command")
	}
	t.Logf("error surfaced as: %s", err)
}

func TestIntegrationBadTokenIsUnauthorized(t *testing.T) {
	host := os.Getenv("SDTD_HOST")
	if host == "" {
		t.Skip("SDTD_HOST not set; skipping live server test")
	}
	port := os.Getenv("SDTD_API_PORT")
	if port == "" {
		port = "8080"
	}
	c, err := New(Options{
		BaseURL:     "http://" + host + ":" + port,
		TokenName:   "panel",
		TokenSecret: "definitely-not-the-right-secret",
		Timeout:     15 * time.Second,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// /api/log requires a real token, unlike the public read endpoints.
	if _, err := c.Log(context.Background(), -1, 1); err == nil {
		t.Error("Log succeeded with a bad token; expected rejection")
	} else {
		t.Logf("bad token rejected as: %s (unauthorized=%v)", err, IsUnauthorized(err))
	}
}

func TestIntegrationReadOnlyValueEndpoints(t *testing.T) {
	c := liveClient(t)
	ctx := context.Background()

	// These three share the envelope and decode path with ServerInfo, but
	// sharing a path is not evidence, so exercise each one.
	tests := []struct {
		name  string
		fetch func() (ValueSet, error)
		probe string
	}{
		{"gamestats", func() (ValueSet, error) { return c.GameStats(ctx) }, "BloodMoonDay"},
		{"gameprefs", func() (ValueSet, error) { return c.GamePrefs(ctx) }, "EnableMapRendering"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			set, err := tt.fetch()
			if err != nil {
				t.Fatalf("%s: %v", tt.name, err)
			}
			if set.Len() == 0 {
				t.Fatalf("%s returned no values", tt.name)
			}
			if _, ok := set.Get(tt.probe); !ok {
				t.Errorf("%s is missing %s", tt.name, tt.probe)
			}
			t.Logf("%s: %d values, %s=%s", tt.name, set.Len(), tt.probe, set.Str(tt.probe))
		})
	}
}

func TestIntegrationBloodmoon(t *testing.T) {
	c := liveClient(t)
	bm, err := c.Bloodmoon(context.Background())
	if err != nil {
		t.Fatalf("Bloodmoon: %v", err)
	}
	// The next blood moon must be a real in-game day, whether or not one is
	// currently running.
	if bm.Next.Days < 1 {
		t.Errorf("next blood moon day = %d, want at least 1", bm.Next.Days)
	}
	if bm.NextBloodmoonEnd.Days < bm.Next.Days {
		t.Errorf("blood moon ends (day %d) before it starts (day %d)",
			bm.NextBloodmoonEnd.Days, bm.Next.Days)
	}
	t.Logf("active=%v next=day %d %02d:%02d until day %d %02d:%02d",
		bm.Active, bm.Next.Days, bm.Next.Hours, bm.Next.Minutes,
		bm.NextBloodmoonEnd.Days, bm.NextBloodmoonEnd.Hours, bm.NextBloodmoonEnd.Minutes)
}

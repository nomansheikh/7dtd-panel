package chat

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/nomansheikh/7dtd-panel/internal/console"
	"github.com/nomansheikh/7dtd-panel/internal/sdtd"
)

/*
Live checks, skipped unless SDTD_HOST is set:

	SDTD_HOST=10.0.0.5 SDTD_API_TOKEN_NAME=panel \
	SDTD_API_TOKEN_SECRET=... go test ./internal/chat -run Integration -v

Only one thing in this package parses text a real server wrote — the admin
list — and it is the thing that decides whether an admins-only command lets
somebody through. Its shape was copied from one live server on one build, which
is exactly the kind of fact that is true until it isn't.

Nothing here writes. The bot's only write is sayplayer, which would put a test
message in front of a real player.
*/
func liveClient(t *testing.T) *sdtd.Client {
	t.Helper()
	host := os.Getenv("SDTD_HOST")
	if host == "" {
		t.Skip("SDTD_HOST not set; skipping live server test")
	}
	port := os.Getenv("SDTD_API_PORT")
	if port == "" {
		port = "8080"
	}
	c, err := sdtd.New(sdtd.Options{
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

func TestIntegrationAdminListIsStillParseable(t *testing.T) {
	client := liveClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	result, err := client.Execute(ctx, console.ListAdmins())
	if err != nil {
		t.Fatalf("admin list: %v", err)
	}
	t.Logf("admin list returned:\n%s", result.Result)

	found := adminID.FindAllString(result.Result, -1)
	t.Logf("platform ids recognised: %v", found)

	// An empty list is a legitimate answer and proves nothing either way, so
	// say so rather than passing quietly.
	if strings.TrimSpace(result.Result) == "" {
		t.Skip("the server returned nothing at all for admin list")
	}
	if len(found) == 0 {
		t.Skip("no admins are defined on this server, so the id format is unverified")
	}
	for _, id := range found {
		if !strings.Contains(id, "_") {
			t.Errorf("recognised %q as a platform id, which does not look like one", id)
		}
	}
}

// The bot answers with sayplayer, which the game accepts only for an entity id
// that is currently connected. This checks the id the panel would use is the
// one the player list reports, without sending anything.
func TestIntegrationOnlinePlayersHaveAnEntityIDToReplyTo(t *testing.T) {
	client := liveClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	players, err := client.Players(ctx)
	if err != nil {
		t.Fatalf("players: %v", err)
	}
	var online int
	for _, p := range players {
		if !p.Online {
			continue
		}
		online++
		if _, err := console.SayPlayer(p.EntityID, "test"); err != nil {
			t.Errorf("%s has entity id %d, which sayplayer would refuse: %v",
				p.Name, p.EntityID, err)
		}
		if p.PlatformID == "" {
			t.Errorf("%s has no platform id, so no cooldown could be recorded", p.Name)
		}
	}
	if online == 0 {
		t.Skip("nobody is online")
	}
	t.Logf("%d online, all addressable", online)
}

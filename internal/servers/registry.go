// Package servers holds the per-server runtime.
//
// Everything that talks to a game server — the client, the poller, the log
// stream, the event hub and the catalogues — exists once per configured
// server. Nothing is shared between them: two servers have different worlds,
// different logs and different credentials, and mixing any of it would show an
// operator one server's data while they believed they were looking at another.
package servers

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/nomansheikh/7dtd-panel/internal/catalog"
	"github.com/nomansheikh/7dtd-panel/internal/config"
	"github.com/nomansheikh/7dtd-panel/internal/events"
	"github.com/nomansheikh/7dtd-panel/internal/gamemap"
	"github.com/nomansheikh/7dtd-panel/internal/power"
	"github.com/nomansheikh/7dtd-panel/internal/sdtd"
	"github.com/nomansheikh/7dtd-panel/internal/state"
)

// The collaborators are interfaces so a test can assemble a Server from fakes
// without standing up a real client, poller and log stream for each one.

// Executor runs console commands and reads the player list.
//
// Console commands are the only write path for game state. Players is here
// rather than on the poller because it merges two endpoints and is read on
// demand, not on a schedule.
type Executor interface {
	Execute(ctx context.Context, command string) (sdtd.CommandResult, error)
	Players(ctx context.Context) ([]sdtd.Player, error)
	GamePrefs(ctx context.Context) (sdtd.ValueSet, error)
	// ReadGamePref reads a preference's live value through the console,
	// because /api/gameprefs does not reflect runtime changes.
	ReadGamePref(ctx context.Context, name string) (string, error)
	// GamePrefsLive does the same for every preference at once.
	GamePrefsLive(ctx context.Context) (map[string]string, error)
	// SandboxSettings supplies the descriptions and allowed values the
	// settings page needs to render pickers instead of text fields.
	SandboxSettings(ctx context.Context) (sdtd.SandboxSettings, error)
	// CurrentWeather reads what the weather is doing, which has no REST
	// endpoint and so comes back through the console.
	CurrentWeather(ctx context.Context) (sdtd.Weather, error)

	// ServerHealth reads load and version figures, which likewise have no REST
	// endpoint of their own.
	ServerHealth(ctx context.Context) (sdtd.Health, error)

	// ItemIcon fetches one item's art, which the panel proxies so the browser
	// never has to reach the game server itself.
	ItemIcon(ctx context.Context, name, tint string) ([]byte, error)

	// MapConfig reports the rendered map's dimensions, which the browser needs
	// before it can place a single tile.
	MapConfig(ctx context.Context) (sdtd.MapConfig, error)
	// Hostiles and Animals are the entities standing in loaded chunks. Both
	// are empty on an idle server: nothing outside a loaded chunk exists.
	Hostiles(ctx context.Context) ([]sdtd.Entity, error)
	Animals(ctx context.Context) ([]sdtd.Entity, error)
	// LandClaims lists every claim block, which is the one overlay that is
	// true whether or not anybody is online.
	LandClaims(ctx context.Context) ([]sdtd.LandClaim, error)
}

// Snapshotter supplies the cached view of a server.
type Snapshotter interface {
	Snapshot() state.Snapshot
}

// CommandCatalogue supplies the server's own command list.
type CommandCatalogue interface {
	Get(ctx context.Context) ([]sdtd.Command, time.Time, error)
}

// ItemCatalogue and EntityCatalogue back the give and spawn pickers.
type ItemCatalogue interface {
	Search(ctx context.Context, query string, includeBlocks bool, limit int) ([]sdtd.Item, int, error)
	Has(ctx context.Context, name string) (bool, error)
}

type EntityCatalogue interface {
	Search(ctx context.Context, query string, spawnableOnly bool, limit int) ([]sdtd.EntityClass, int, error)
	Lookup(ctx context.Context, name string) (sdtd.EntityClass, bool, error)
}

// BuffCatalogue backs the buff picker and rejects names the server would not
// recognise.
type BuffCatalogue interface {
	Search(ctx context.Context, query string, limit int) ([]sdtd.Buff, int, error)
	Has(ctx context.Context, name string) (bool, error)
}

// EventFeed is the hub browser clients subscribe to, and the one place the
// panel can speak into its own feed.
//
// PublishStatus is here rather than on a separate collaborator because the
// things that need it — the poller announcing that a server went away, the chat
// bot announcing that somebody was handed a kit — are both writing into the
// same stream an operator is already watching.
type EventFeed interface {
	Subscribe(backlog int) ([]events.Event, <-chan events.Event, func())
	PublishStatus(message string)
}

// Server is one game server and everything the panel keeps for it.
type Server struct {
	ID   string
	Name string
	// BaseURL is exposed for diagnostics. It contains no credentials.
	BaseURL string

	Client   Executor
	Poller   Snapshotter
	Events   EventFeed
	Commands CommandCatalogue
	Items    ItemCatalogue
	Entities EntityCatalogue
	Buffs    BuffCatalogue
	// Power runs the stop sequence for this server and watches whether it
	// comes back. One per server, because only one can be stopping at a time.
	Power *power.Controller
	// Map holds this server's rendered tiles. Per server because two servers
	// have different worlds, and a tile is identified only by its coordinates.
	Map *gamemap.Cache

	// These are the concrete collaborators Run needs. A Server assembled by a
	// test leaves them nil and is simply never Run.
	runner  *state.Poller
	stream  *sdtd.LogStreamer
	publish func(sdtd.LogEntry)
	warm    func(context.Context)
}

/*
build assembles one server's runtime. It contacts nothing: the panel must start
and serve a clear disconnected state even when every game server is down.
*/
func build(
	gc config.Game, log *slog.Logger, pollInterval time.Duration,
	failureThreshold int, allowDestructive bool,
) (*Server, error) {
	serverLog := log.With("server", gc.ID)

	client, err := sdtd.New(sdtd.Options{
		BaseURL:     gc.BaseURL(),
		TokenName:   gc.TokenName,
		TokenSecret: gc.TokenSecret,
		Timeout:     10 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("server %s: %w", gc.ID, err)
	}

	hub := events.NewHub()
	items := catalog.NewItems(client, time.Hour, serverLog.With("component", "catalog"))
	srv := &Server{
		ID:       gc.ID,
		Name:     gc.Name,
		BaseURL:  gc.BaseURL(),
		Client:   client,
		Events:   hub,
		Commands: catalog.NewCommands(client, time.Hour, serverLog.With("component", "catalog")),
		Items:    items,
		Entities: catalog.NewEntities(client, time.Hour, serverLog.With("component", "catalog")),
		Buffs:    catalog.NewBuffs(client, time.Hour, serverLog.With("component", "catalog")),
		stream:   sdtd.NewLogStreamer(client, serverLog.With("component", "logstream")),
		publish:  hub.PublishLog,
		warm:     items.Warm,
	}

	poller := state.New(state.Options{
		Client:           client,
		Interval:         pollInterval,
		FailureThreshold: failureThreshold,
		Logger:           serverLog.With("component", "poller"),
		OnStatusChange: func(from, to state.Status) {
			hub.PublishStatus("Game server " + string(to) + " (was " + string(from) + ")")
		},
	})
	srv.Poller = poller
	srv.runner = poller
	srv.Power = power.New(power.Options{
		Server:           gc.ID,
		Client:           client,
		Poller:           poller,
		Announce:         hub,
		AllowDestructive: allowDestructive,
	})
	// The map cache asks the poller whether anybody is in the world, because
	// the renderer only redraws a tile while a player is loading chunks. On an
	// empty server the map is frozen and can be trusted for far longer.
	srv.Map = gamemap.New(gamemap.Options{
		Source: client,
		Busy:   func() bool { return poller.Snapshot().CurrentPlayers > 0 },
	})
	return srv, nil
}

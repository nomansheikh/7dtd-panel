package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/nomansheikh/7dtd-panel/internal/config"
	"github.com/nomansheikh/7dtd-panel/internal/servers"
	"github.com/nomansheikh/7dtd-panel/internal/store"
)

/*
Getting the configured game servers into the registry at boot.

The database is the source of truth. The environment is a way in for anyone
whose panel is managed as code, and a migration path for every install that
predates servers being addable from the UI — so anything configured there is
copied in once, on the first boot that finds the table empty of it, and then
left alone. Editing a server in the panel afterwards is not undone by the
environment still saying something older.
*/
func loadServers(
	ctx context.Context, db *store.Store, registry *servers.Registry,
	cfg config.Config, log *slog.Logger,
) error {
	if err := seedServersFromEnv(ctx, db, cfg, log); err != nil {
		return err
	}

	saved, err := db.GameServers(ctx)
	if err != nil {
		return err
	}
	for _, g := range saved {
		if _, err := registry.Add(asGame(g)); err != nil {
			// One unusable row must not stop the panel: an operator needs to
			// get in to fix it, and they cannot do that if it will not start.
			log.Error("game server could not be started", "server", g.ID, "error", err)
			continue
		}
		log.Info("game server configured", "id", g.ID, "name", g.Name)
	}

	if len(registry.All()) == 0 {
		log.Info("no game servers yet; add one from the panel")
	}
	return nil
}

// seedServersFromEnv copies anything in the environment into the database, once
// per server id. An id already in the table is left as it is.
func seedServersFromEnv(
	ctx context.Context, db *store.Store, cfg config.Config, log *slog.Logger,
) error {
	for i, gc := range cfg.Servers {
		if _, err := db.GameServer(ctx, gc.ID); err == nil {
			continue
		} else if err != store.ErrNotFound {
			return err
		}
		err := db.SaveGameServer(ctx, store.GameServer{
			ID:          gc.ID,
			Name:        gc.Name,
			Host:        gc.Host,
			Port:        gc.Port,
			Scheme:      gc.Scheme,
			TokenName:   gc.TokenName,
			TokenSecret: gc.TokenSecret,
			Position:    i,
		}, time.Now())
		if err != nil {
			return err
		}
		log.Info("game server imported from the environment", "id", gc.ID)
	}
	return nil
}

// asGame turns a stored server into the shape the registry builds from.
func asGame(g store.GameServer) config.Game {
	return config.Game{
		ID:          g.ID,
		Name:        g.Name,
		Host:        g.Host,
		Port:        g.Port,
		Scheme:      g.Scheme,
		TokenName:   g.TokenName,
		TokenSecret: g.TokenSecret,
	}
}

// Command 7dtd-panel is a self-hostable admin panel for 7 Days to Die
// dedicated servers.
//
// One process serves the API and the embedded UI. The API token for the game
// server lives only here; the browser never talks to the game server directly.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/nomansheikh/7dtd-panel/internal/api"
	"github.com/nomansheikh/7dtd-panel/internal/auth"
	"github.com/nomansheikh/7dtd-panel/internal/catalog"
	"github.com/nomansheikh/7dtd-panel/internal/config"
	"github.com/nomansheikh/7dtd-panel/internal/events"
	"github.com/nomansheikh/7dtd-panel/internal/httpx"
	"github.com/nomansheikh/7dtd-panel/internal/sdtd"
	"github.com/nomansheikh/7dtd-panel/internal/state"
	"github.com/nomansheikh/7dtd-panel/internal/store"
	"github.com/nomansheikh/7dtd-panel/internal/web"
)

// version is set at build time with -ldflags "-X main.version=vX.Y.Z".
var version = "dev"

func main() {
	// healthcheck runs in the same binary so the distroless image needs no
	// shell or curl for its HEALTHCHECK.
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		os.Exit(healthcheck())
	}
	if len(os.Args) > 1 && (os.Args[1] == "version" || os.Args[1] == "--version") {
		fmt.Println(version)
		return
	}

	if err := run(); err != nil {
		// The logger may not exist yet, so write plainly to stderr.
		fmt.Fprintln(os.Stderr, "fatal: "+err.Error())
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}

	log := newLogger(cfg.Panel.LogLevel, cfg.Panel.LogFormat)
	slog.SetDefault(log)

	log.Info("starting 7dtd-panel", "version", version)
	for k, v := range cfg.Redacted() {
		log.Debug("config", k, v)
	}

	// Signal handling is installed before anything slow, so an early Ctrl-C is
	// honoured rather than ignored.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := store.Open(ctx, cfg.Panel.DBPath)
	if err != nil {
		return err
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Warn("closing database failed", "error", err)
		}
	}()

	applied, err := db.AppliedMigrations(ctx)
	if err != nil {
		return err
	}
	log.Info("database ready", "path", cfg.Panel.DBPath, "migrations", len(applied))

	if err := seedAdmin(ctx, db, cfg, log); err != nil {
		return err
	}

	// The client is constructed without contacting the server, so the panel
	// starts and serves a clear disconnected state even if the game server is
	// down.
	gameClient, err := sdtd.New(sdtd.Options{
		BaseURL:     cfg.Game.BaseURL(),
		TokenName:   cfg.Game.TokenName,
		TokenSecret: cfg.Game.TokenSecret,
		Timeout:     10 * time.Second,
	})
	if err != nil {
		return err
	}

	// One hub fans the game server's log out to every browser tab, so the
	// number of connections to the game server does not grow with viewers.
	hub := events.NewHub()

	poller := state.New(state.Options{
		Client:           gameClient,
		Interval:         cfg.Panel.PollInterval,
		FailureThreshold: cfg.Panel.FailureThreshold,
		Logger:           log.With("component", "poller"),
		OnStatusChange: func(from, to state.Status) {
			hub.PublishStatus("Game server " + string(to) + " (was " + string(from) + ")")
		},
	})

	catalogLog := log.With("component", "catalog")
	commandCatalogue := catalog.NewCommands(gameClient, time.Hour, catalogLog)
	itemCatalogue := catalog.NewItems(gameClient, time.Hour, catalogLog)
	entityCatalogue := catalog.NewEntities(gameClient, time.Hour, catalogLog)

	logStream := sdtd.NewLogStreamer(gameClient, log.With("component", "logstream"))

	sessions := auth.NewSessions(db, cfg.Panel.SessionTTL, cfg.Panel.TrustProxy)

	apiServer := api.NewServer(api.Deps{
		Config:   cfg,
		Store:    db,
		Sessions: sessions,
		State:    poller,
		Game:     gameClient,
		Commands: commandCatalogue,
		Items:    itemCatalogue,
		Entities: entityCatalogue,
		Events:   hub,
		Logger:   log.With("component", "api"),
		Version:  version,
	})

	// The poller and the session sweeper run for the life of the process and
	// stop when ctx is cancelled.
	go poller.Run(ctx)
	go apiServer.CleanupSessions(ctx, time.Hour)
	go logStream.Run(ctx, hub.PublishLog)
	// The item catalogue is ~2.9 MB, so it is pulled in the background rather
	// than making the first picker keystroke wait for it.
	go itemCatalogue.Warm(ctx)
	go pruneHistory(ctx, db, log)

	mux := http.NewServeMux()
	mux.Handle("/api/", apiServer.Routes())
	mux.Handle("/", web.Handler())

	handler := httpx.Chain(mux,
		httpx.RequestID,
		httpx.Logger(log.With("component", "http")),
		httpx.Recover(log.With("component", "http")),
		httpx.SecurityHeaders,
	)

	addr := net.JoinHostPort("", strconv.Itoa(cfg.Panel.Port))
	srv := &http.Server{
		Addr:    addr,
		Handler: handler,
		// Generous read and idle timeouts; no write timeout, because the
		// streaming endpoint added in the next phase holds a response open
		// indefinitely and a WriteTimeout would sever it.
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	if _, err := web.FS(); errors.Is(err, web.ErrNotBuilt) {
		log.Warn("no frontend build embedded; the API works but the UI will not load",
			"hint", "run make frontend")
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
		log.Info("shutdown signal received")
	}

	// Give in-flight requests a chance to finish before dropping them.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Warn("graceful shutdown timed out", "error", err)
		_ = srv.Close()
	}
	log.Info("stopped")
	return nil
}

// seedAdmin reconciles the admin account with PANEL_ADMIN_PASSWORD on every
// boot. See store.UpsertAdmin for why this is not first-run-only.
func seedAdmin(ctx context.Context, db *store.Store, cfg config.Config, log *slog.Logger) error {
	hash, err := auth.HashPassword(cfg.Panel.AdminPassword)
	if err != nil {
		return err
	}

	existing, err := db.UserByUsername(ctx, cfg.Panel.AdminUsername)
	switch {
	case errors.Is(err, store.ErrNotFound):
		if _, _, err := db.UpsertAdmin(ctx, cfg.Panel.AdminUsername, hash); err != nil {
			return err
		}
		log.Info("admin account created", "username", cfg.Panel.AdminUsername)
		return nil
	case err != nil:
		return err
	}

	// Re-hashing produces a different salt every time, so comparing hashes
	// would always differ. Verify instead, and only write when the configured
	// password no longer matches what is stored.
	ok, verifyErr := auth.VerifyPassword(existing.PasswordHash, cfg.Panel.AdminPassword)
	if verifyErr == nil && ok && !auth.NeedsRehash(existing.PasswordHash) {
		log.Debug("admin password unchanged")
		return nil
	}

	if _, _, err := db.UpsertAdmin(ctx, cfg.Panel.AdminUsername, hash); err != nil {
		return err
	}
	log.Info("admin password updated from the environment; existing sessions were revoked",
		"username", cfg.Panel.AdminUsername)
	return nil
}

func newLogger(level, format string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: lvl}

	if format == "text" {
		return slog.New(slog.NewTextHandler(os.Stderr, opts))
	}
	return slog.New(slog.NewJSONHandler(os.Stderr, opts))
}

// healthcheck probes the panel's own health endpoint. It is the container
// HEALTHCHECK, and reports only on the panel: a game server that is down must
// not make the orchestrator restart the panel.
func healthcheck() int {
	port := os.Getenv("PANEL_PORT")
	if port == "" {
		port = "8080"
	}
	client := &http.Client{Timeout: 5 * time.Second}
	url := "http://127.0.0.1:" + port + "/api/health"

	resp, err := client.Get(url)
	if err != nil {
		fmt.Fprintln(os.Stderr, "healthcheck: "+err.Error())
		return 1
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "healthcheck: %s returned %d\n", url, resp.StatusCode)
		return 1
	}
	return 0
}

// historyKeep is how many console commands are retained per operator.
const historyKeep = 500

// pruneHistory trims console history so a long-lived panel does not accumulate
// it without bound.
func pruneHistory(ctx context.Context, db *store.Store, log *slog.Logger) {
	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n, err := db.PruneCommandHistory(ctx, historyKeep)
			if err != nil {
				log.Warn("pruning command history failed", "error", err)
				continue
			}
			if n > 0 {
				log.Debug("pruned command history", "removed", n)
			}
		}
	}
}

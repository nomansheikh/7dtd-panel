// Package catalog caches the large, slow-changing lists the game server
// exposes, so the panel does not refetch them on every request.
package catalog

import (
	"context"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/nomansheikh/7dtd-panel/internal/sdtd"
)

// Fetcher is the slice of the game client this package needs.
type Fetcher interface {
	Commands(ctx context.Context) ([]sdtd.Command, error)
}

// Commands caches the server's command catalogue.
//
// The list is ~55 KB and changes only when mods change, so refetching it per
// keystroke in the palette would be wasteful. It is deliberately not fetched at
// construction: the panel must start even when the game server is down.
type Commands struct {
	client Fetcher
	ttl    time.Duration
	log    *slog.Logger
	now    func() time.Time

	mu        sync.RWMutex
	items     []sdtd.Command
	fetchedAt time.Time
	// inflight collapses concurrent refreshes into one request.
	inflight sync.Mutex
}

// NewCommands builds the cache. A zero ttl selects one hour.
func NewCommands(client Fetcher, ttl time.Duration, log *slog.Logger) *Commands {
	if ttl <= 0 {
		ttl = time.Hour
	}
	if log == nil {
		log = slog.Default()
	}
	return &Commands{client: client, ttl: ttl, log: log, now: time.Now}
}

// Get returns the catalogue, fetching it if absent or stale.
//
// When a refresh fails but a previous copy exists, that copy is returned with
// no error: a momentarily unreachable game server should not empty the command
// palette.
func (c *Commands) Get(ctx context.Context) ([]sdtd.Command, time.Time, error) {
	c.mu.RLock()
	items, at := c.items, c.fetchedAt
	c.mu.RUnlock()

	if items != nil && c.now().Sub(at) < c.ttl {
		return items, at, nil
	}

	c.inflight.Lock()
	defer c.inflight.Unlock()

	// Another goroutine may have refreshed while this one waited.
	c.mu.RLock()
	items, at = c.items, c.fetchedAt
	c.mu.RUnlock()
	if items != nil && c.now().Sub(at) < c.ttl {
		return items, at, nil
	}

	fetched, err := c.client.Commands(ctx)
	if err != nil {
		if items != nil {
			c.log.Warn("command catalogue refresh failed; serving the cached copy",
				"error", err, "cachedAt", at)
			return items, at, nil
		}
		return nil, time.Time{}, err
	}

	sort.Slice(fetched, func(i, j int) bool {
		return strings.ToLower(fetched[i].Command) < strings.ToLower(fetched[j].Command)
	})

	now := c.now()
	c.mu.Lock()
	c.items, c.fetchedAt = fetched, now
	c.mu.Unlock()

	c.log.Debug("command catalogue refreshed", "commands", len(fetched))
	return fetched, now, nil
}

// Invalidate forces the next Get to refetch.
func (c *Commands) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items, c.fetchedAt = nil, time.Time{}
}

// Lookup finds a command by its primary name or any alias, case-insensitively.
// It only consults what is already cached.
func (c *Commands) Lookup(name string) (sdtd.Command, bool) {
	needle := strings.ToLower(strings.TrimSpace(name))
	if needle == "" {
		return sdtd.Command{}, false
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, item := range c.items {
		if strings.ToLower(item.Command) == needle {
			return item, true
		}
		for _, alias := range item.Overloads {
			if strings.ToLower(alias) == needle {
				return item, true
			}
		}
	}
	return sdtd.Command{}, false
}

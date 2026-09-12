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

// maxSearchLimit is high enough to hand the whole catalogue over at once. Both
// lists are a few hundred entries, cached, and static for the life of the
// server, so a picker is better off holding all of it than round-tripping on
// every keystroke.
const maxSearchLimit = 2000

// clampLimit keeps a requested page size inside the bounds.
//
// Clamped rather than reset: asking for more than the ceiling used to hand back
// fifty, so a caller wanting everything silently got one page of it with no way
// to tell the difference.
func clampLimit(limit int) int {
	switch {
	case limit <= 0:
		return 50
	case limit > maxSearchLimit:
		return maxSearchLimit
	default:
		return limit
	}
}

// ItemFetcher and EntityFetcher are the slices of the game client needed here.
type ItemFetcher interface {
	Items(ctx context.Context) ([]sdtd.Item, error)
}

type EntityFetcher interface {
	EntityClasses(ctx context.Context) ([]sdtd.EntityClass, error)
}

// Items caches and searches the item catalogue.
//
// The full list is about 2.9 MB and several thousand entries. It is never sent
// to the browser: the picker asks for a short ranked slice instead.
type Items struct {
	client ItemFetcher
	ttl    time.Duration
	log    *slog.Logger

	mu        sync.RWMutex
	items     []sdtd.Item
	fetchedAt time.Time
	inflight  sync.Mutex
}

func NewItems(client ItemFetcher, ttl time.Duration, log *slog.Logger) *Items {
	if ttl <= 0 {
		ttl = time.Hour
	}
	if log == nil {
		log = slog.Default()
	}
	return &Items{client: client, ttl: ttl, log: log}
}

// Warm loads the catalogue in the background.
//
// Called at startup so the first picker keystroke is not waiting on a 2.9 MB
// download, and deliberately tolerant of failure: the panel must start even
// when the game server is down.
func (i *Items) Warm(ctx context.Context) {
	if _, err := i.all(ctx); err != nil {
		i.log.Warn("could not preload the item catalogue", "error", err)
	}
}

func (i *Items) all(ctx context.Context) ([]sdtd.Item, error) {
	i.mu.RLock()
	items, at := i.items, i.fetchedAt
	i.mu.RUnlock()
	if items != nil && time.Since(at) < i.ttl {
		return items, nil
	}

	i.inflight.Lock()
	defer i.inflight.Unlock()

	i.mu.RLock()
	items, at = i.items, i.fetchedAt
	i.mu.RUnlock()
	if items != nil && time.Since(at) < i.ttl {
		return items, nil
	}

	fetched, err := i.client.Items(ctx)
	if err != nil {
		if items != nil {
			// A stale catalogue beats an empty picker.
			return items, nil
		}
		return nil, err
	}

	i.mu.Lock()
	i.items, i.fetchedAt = fetched, time.Now()
	i.mu.Unlock()
	i.log.Debug("item catalogue loaded", "items", len(fetched))
	return fetched, nil
}

// Search returns items matching query, best first, capped at limit.
func (i *Items) Search(ctx context.Context, query string, includeBlocks bool, limit int) ([]sdtd.Item, int, error) {
	all, err := i.all(ctx)
	if err != nil {
		return nil, 0, err
	}
	limit = clampLimit(limit)

	needle := strings.ToLower(strings.TrimSpace(query))
	matched := make([]sdtd.Item, 0, limit)
	for _, item := range all {
		if !includeBlocks && item.IsBlock {
			continue
		}
		if needle != "" &&
			!strings.Contains(strings.ToLower(item.Name), needle) &&
			!strings.Contains(strings.ToLower(item.LocalizedName), needle) {
			continue
		}
		matched = append(matched, item)
	}

	total := len(matched)
	// A prefix match is almost always what someone typing a name wants, so it
	// outranks a match buried in the middle of a longer name.
	if needle != "" {
		sort.SliceStable(matched, func(a, b int) bool {
			return rank(matched[a], needle) < rank(matched[b], needle)
		})
	}
	if len(matched) > limit {
		matched = matched[:limit]
	}
	return matched, total, nil
}

func rank(item sdtd.Item, needle string) int {
	name := strings.ToLower(item.Name)
	local := strings.ToLower(item.LocalizedName)
	switch {
	case name == needle || local == needle:
		return 0
	case strings.HasPrefix(name, needle) || strings.HasPrefix(local, needle):
		return 1
	default:
		return 2
	}
}

// Has reports whether an exact item name exists, which is how a give request is
// validated before it becomes a command.
func (i *Items) Has(ctx context.Context, name string) (bool, error) {
	all, err := i.all(ctx)
	if err != nil {
		return false, err
	}
	for _, item := range all {
		if item.Name == name {
			return true, nil
		}
	}
	return false, nil
}

// Entities caches and searches the entity class catalogue.
type Entities struct {
	client EntityFetcher
	ttl    time.Duration
	log    *slog.Logger

	mu        sync.RWMutex
	items     []sdtd.EntityClass
	fetchedAt time.Time
	inflight  sync.Mutex
}

func NewEntities(client EntityFetcher, ttl time.Duration, log *slog.Logger) *Entities {
	if ttl <= 0 {
		ttl = time.Hour
	}
	if log == nil {
		log = slog.Default()
	}
	return &Entities{client: client, ttl: ttl, log: log}
}

func (e *Entities) all(ctx context.Context) ([]sdtd.EntityClass, error) {
	e.mu.RLock()
	items, at := e.items, e.fetchedAt
	e.mu.RUnlock()
	if items != nil && time.Since(at) < e.ttl {
		return items, nil
	}

	e.inflight.Lock()
	defer e.inflight.Unlock()

	e.mu.RLock()
	items, at = e.items, e.fetchedAt
	e.mu.RUnlock()
	if items != nil && time.Since(at) < e.ttl {
		return items, nil
	}

	fetched, err := e.client.EntityClasses(ctx)
	if err != nil {
		if items != nil {
			return items, nil
		}
		return nil, err
	}

	e.mu.Lock()
	e.items, e.fetchedAt = fetched, time.Now()
	e.mu.Unlock()
	return fetched, nil
}

// Search returns spawnable entity classes matching query.
//
// Classes the game marks as not manually spawnable are excluded by default:
// offering them would produce a command that always fails.
func (e *Entities) Search(ctx context.Context, query string, spawnableOnly bool, limit int) ([]sdtd.EntityClass, int, error) {
	all, err := e.all(ctx)
	if err != nil {
		return nil, 0, err
	}
	limit = clampLimit(limit)

	needle := strings.ToLower(strings.TrimSpace(query))
	matched := make([]sdtd.EntityClass, 0, limit)
	for _, item := range all {
		if spawnableOnly && item.ManualSpawnType == "None" {
			continue
		}
		if needle != "" && !strings.Contains(strings.ToLower(item.Name), needle) {
			continue
		}
		matched = append(matched, item)
	}

	total := len(matched)
	if len(matched) > limit {
		matched = matched[:limit]
	}
	return matched, total, nil
}

// Lookup finds an entity class by exact name.
func (e *Entities) Lookup(ctx context.Context, name string) (sdtd.EntityClass, bool, error) {
	all, err := e.all(ctx)
	if err != nil {
		return sdtd.EntityClass{}, false, err
	}
	for _, item := range all {
		if item.Name == name {
			return item, true, nil
		}
	}
	return sdtd.EntityClass{}, false, nil
}

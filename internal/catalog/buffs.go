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

// BuffFetcher is the slice of the game client this cache needs.
type BuffFetcher interface {
	Buffs(ctx context.Context) ([]sdtd.Buff, error)
}

// Buffs caches and searches the buff catalogue.
//
// Fetching it costs a console command, so it is cached like the others rather
// than run on every keystroke.
type Buffs struct {
	client BuffFetcher
	ttl    time.Duration
	log    *slog.Logger

	mu        sync.RWMutex
	items     []sdtd.Buff
	fetchedAt time.Time
	inflight  sync.Mutex
}

func NewBuffs(client BuffFetcher, ttl time.Duration, log *slog.Logger) *Buffs {
	if ttl <= 0 {
		ttl = time.Hour
	}
	if log == nil {
		log = slog.Default()
	}
	return &Buffs{client: client, ttl: ttl, log: log}
}

func (b *Buffs) all(ctx context.Context) ([]sdtd.Buff, error) {
	b.mu.RLock()
	items, at := b.items, b.fetchedAt
	b.mu.RUnlock()
	if items != nil && time.Since(at) < b.ttl {
		return items, nil
	}

	b.inflight.Lock()
	defer b.inflight.Unlock()

	b.mu.RLock()
	items, at = b.items, b.fetchedAt
	b.mu.RUnlock()
	if items != nil && time.Since(at) < b.ttl {
		return items, nil
	}

	fetched, err := b.client.Buffs(ctx)
	if err != nil {
		if items != nil {
			// A stale catalogue beats an empty picker.
			return items, nil
		}
		return nil, err
	}

	b.mu.Lock()
	b.items, b.fetchedAt = fetched, time.Now()
	b.mu.Unlock()
	b.log.Debug("buff catalogue loaded", "buffs", len(fetched))
	return fetched, nil
}

// Search returns buffs matching query, best first, capped at limit.
func (b *Buffs) Search(ctx context.Context, query string, limit int) ([]sdtd.Buff, int, error) {
	all, err := b.all(ctx)
	if err != nil {
		return nil, 0, err
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	needle := strings.ToLower(strings.TrimSpace(query))
	matched := make([]sdtd.Buff, 0, limit)
	for _, buff := range all {
		if needle != "" &&
			!strings.Contains(strings.ToLower(buff.Name), needle) &&
			!strings.Contains(strings.ToLower(buff.LocalizedName), needle) {
			continue
		}
		matched = append(matched, buff)
	}

	total := len(matched)
	if needle != "" {
		// A buff whose display name starts with what was typed is almost
		// always the one meant.
		sort.SliceStable(matched, func(i, j int) bool {
			return buffRank(matched[i], needle) < buffRank(matched[j], needle)
		})
	}
	if len(matched) > limit {
		matched = matched[:limit]
	}
	return matched, total, nil
}

func buffRank(b sdtd.Buff, needle string) int {
	switch {
	case strings.HasPrefix(strings.ToLower(b.LocalizedName), needle):
		return 0
	case strings.HasPrefix(strings.ToLower(b.Name), needle):
		return 1
	case strings.Contains(strings.ToLower(b.LocalizedName), needle):
		return 2
	default:
		return 3
	}
}

// Has reports whether a buff of exactly this name exists.
//
// Used to refuse a name the server would not recognise before it becomes a
// command, rather than after.
func (b *Buffs) Has(ctx context.Context, name string) (bool, error) {
	all, err := b.all(ctx)
	if err != nil {
		return false, err
	}
	for _, buff := range all {
		if strings.EqualFold(buff.Name, name) {
			return true, nil
		}
	}
	return false, nil
}

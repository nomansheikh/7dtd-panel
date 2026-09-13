package gamemap

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/nomansheikh/7dtd-panel/internal/sdtd"
)

// What happens when the cache has to go and ask: coalescing duplicate
// requests, remembering blanks, and holding the last good tile through an
// outage. The shared fakes live in cache_test.go.

func TestConcurrentRequestsForOneTileShareASingleFetch(t *testing.T) {
	src := &fakeSource{body: []byte("tile"), block: make(chan struct{})}
	cache, _ := newCache(t, src, nil)

	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := cache.Get(context.Background(), Key{Z: 1, X: 1, Y: 1}); err != nil {
				t.Errorf("Get: %v", err)
			}
		}()
	}
	// Let them all arrive and coalesce before the fetch is allowed to finish.
	time.Sleep(50 * time.Millisecond)
	close(src.block)
	wg.Wait()

	if src.count() != 1 {
		t.Fatalf("asked the game server %d times for one tile, want 1", src.count())
	}
}

func TestATileKeepsItsETagWhenTheBytesHaveNotChanged(t *testing.T) {
	src := &fakeSource{body: []byte("tile")}
	cache, clk := newCache(t, src, func(o *Options) { o.BusyTTL = time.Second })

	key := Key{Z: 3, X: 0, Y: 0}
	first, err := cache.Get(context.Background(), key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	clk.add(2 * time.Second)
	second, err := cache.Get(context.Background(), key)
	if err != nil {
		t.Fatalf("Get after expiry: %v", err)
	}
	if first.ETag != second.ETag {
		t.Fatalf("ETag changed from %s to %s for identical bytes", first.ETag, second.ETag)
	}
	if first.ETag == "" {
		t.Fatal("want an ETag")
	}
}

// Degrade, don't crash: a blip should leave the map as it was.
func TestALastKnownTileIsHeldThroughAnOutage(t *testing.T) {
	src := &fakeSource{body: []byte("tile")}
	cache, clk := newCache(t, src, func(o *Options) { o.BusyTTL = time.Second })

	key := Key{Z: 4, X: 5, Y: 6}
	if _, err := cache.Get(context.Background(), key); err != nil {
		t.Fatalf("Get: %v", err)
	}

	src.set(nil, errors.New("connection refused"))
	clk.add(2 * time.Second)

	tile, err := cache.Get(context.Background(), key)
	if err != nil {
		t.Fatalf("Get during outage: %v", err)
	}
	if string(tile.Bytes) != "tile" {
		t.Fatalf("got %q, want the last known tile", tile.Bytes)
	}
}

func TestATileNeverSeenReportsTheOutage(t *testing.T) {
	src := &fakeSource{err: errors.New("connection refused")}
	cache, _ := newCache(t, src, nil)

	if _, err := cache.Get(context.Background(), Key{Z: 0, X: 0, Y: 0}); err == nil {
		t.Fatal("want an error when there is nothing cached to fall back on")
	}
}

func TestTheCacheStaysUnderItsByteLimit(t *testing.T) {
	src := &fakeSource{body: make([]byte, 1000)}
	cache, _ := newCache(t, src, func(o *Options) { o.MaxBytes = 5000 })

	for x := range 20 {
		if _, err := cache.Get(context.Background(), Key{Z: 4, X: x, Y: 0}); err != nil {
			t.Fatalf("Get: %v", err)
		}
	}

	tiles, bytes := cache.Stats()
	if bytes > 5000 {
		t.Fatalf("holding %d bytes, over the 5000 limit", bytes)
	}
	if tiles != 5 {
		t.Fatalf("holding %d tiles, want 5", tiles)
	}
}

// The first fetch is the one that matters: if it comes back without a max-age
// the browser refetches the whole viewport on the next pan, and the cache only
// ever helps the panel, never the person looking at it.
func TestAFreshlyFetchedTileCarriesHowLongItMayBeReused(t *testing.T) {
	src := &fakeSource{body: []byte("tile")}
	players := 0
	cache, _ := newCache(t, src, func(o *Options) {
		o.Busy = func() bool { return players > 0 }
		o.BusyTTL = 30 * time.Second
		o.IdleTTL = 30 * time.Minute
	})

	tile, err := cache.Get(context.Background(), Key{Z: 4, X: 0, Y: 0})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if tile.MaxAge != 30*time.Minute {
		t.Fatalf("MaxAge %v on an idle server, want 30m", tile.MaxAge)
	}

	// A square the renderer has never drawn needs one too, or the browser
	// re-asks for every blank tile on every pan.
	blanks := &fakeSource{err: sdtd.ErrNoTile}
	blankCache, _ := newCache(t, blanks, func(o *Options) { o.IdleTTL = 30 * time.Minute })
	blank, err := blankCache.Get(context.Background(), Key{Z: 4, X: 1, Y: 1})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if blank.MaxAge <= 0 {
		t.Fatalf("MaxAge %v on an unrendered square, want it cacheable", blank.MaxAge)
	}
}

// Most of an unexplored world is missing. Re-asking for every blank square on
// every pan is what makes a fresh map unusable.
func TestAnUnrenderedSquareIsRememberedRatherThanReasked(t *testing.T) {
	src := &fakeSource{err: sdtd.ErrNoTile}
	cache, _ := newCache(t, src, nil)

	for range 4 {
		tile, err := cache.Get(context.Background(), Key{Z: 4, X: 9, Y: 9})
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if !tile.Missing {
			t.Fatal("want the tile reported missing")
		}
	}
	if src.count() != 1 {
		t.Fatalf("asked the game server %d times for a square it has never drawn, want 1", src.count())
	}
}

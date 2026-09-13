package gamemap

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nomansheikh/7dtd-panel/internal/sdtd"
)

// fakeSource counts calls so a test can tell a cache hit from a fetch.
type fakeSource struct {
	mu    sync.Mutex
	calls int32
	body  []byte
	err   error
	// block, when set, holds every fetch until it is closed.
	block chan struct{}
}

func (f *fakeSource) MapTile(_ context.Context, _, x, y int) ([]byte, error) {
	// The cache probes a square no world contains to learn what this server
	// sends for ground it has not drawn. Answer the way a server with
	// rendering off does, and leave it out of the count: these tests are about
	// how often a real tile is fetched.
	if x >= blankProbe || y >= blankProbe {
		return nil, sdtd.ErrNoTile
	}
	atomic.AddInt32(&f.calls, 1)
	if f.block != nil {
		<-f.block
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	return f.body, nil
}

func (f *fakeSource) count() int { return int(atomic.LoadInt32(&f.calls)) }

func (f *fakeSource) set(body []byte, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.body, f.err = body, err
}

type clock struct{ t time.Time }

func (c *clock) now() time.Time      { return c.t }
func (c *clock) add(d time.Duration) { c.t = c.t.Add(d) }

func newCache(t *testing.T, src Source, opts func(*Options)) (*Cache, *clock) {
	t.Helper()
	c := &clock{t: time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)}
	o := Options{Source: src, Now: c.now}
	if opts != nil {
		opts(&o)
	}
	return New(o), c
}

func TestASecondRequestForTheSameTileIsServedFromMemory(t *testing.T) {
	src := &fakeSource{body: []byte("tile")}
	cache, _ := newCache(t, src, nil)

	for range 5 {
		tile, err := cache.Get(context.Background(), Key{Z: 4, X: 1, Y: 2})
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if string(tile.Bytes) != "tile" {
			t.Fatalf("got %q, want %q", tile.Bytes, "tile")
		}
	}
	if src.count() != 1 {
		t.Fatalf("asked the game server %d times, want 1", src.count())
	}
}

func TestATileIsRefetchedOnceItsTTLHasPassed(t *testing.T) {
	src := &fakeSource{body: []byte("tile")}
	cache, clk := newCache(t, src, func(o *Options) { o.BusyTTL = time.Minute })

	key := Key{Z: 0, X: 0, Y: 0}
	if _, err := cache.Get(context.Background(), key); err != nil {
		t.Fatalf("Get: %v", err)
	}
	clk.add(time.Minute + time.Second)
	if _, err := cache.Get(context.Background(), key); err != nil {
		t.Fatalf("Get after expiry: %v", err)
	}
	if src.count() != 2 {
		t.Fatalf("asked the game server %d times, want 2", src.count())
	}
}

// An empty world cannot redraw a tile, so the panel should stop asking.
func TestAnIdleServersTilesAreTrustedForLonger(t *testing.T) {
	src := &fakeSource{body: []byte("tile")}
	players := 0
	cache, clk := newCache(t, src, func(o *Options) {
		o.Busy = func() bool { return players > 0 }
		o.BusyTTL = 30 * time.Second
		o.IdleTTL = 30 * time.Minute
	})

	key := Key{Z: 2, X: 3, Y: 4}
	if _, err := cache.Get(context.Background(), key); err != nil {
		t.Fatalf("Get: %v", err)
	}
	clk.add(10 * time.Minute)
	if _, err := cache.Get(context.Background(), key); err != nil {
		t.Fatalf("Get while idle: %v", err)
	}
	if src.count() != 1 {
		t.Fatalf("asked the game server %d times while nobody was playing, want 1", src.count())
	}

	// Somebody joins: the same age is now too old to trust.
	players = 1
	if _, err := cache.Get(context.Background(), key); err != nil {
		t.Fatalf("Get once busy: %v", err)
	}
	if src.count() != 2 {
		t.Fatalf("asked the game server %d times once a player joined, want 2", src.count())
	}
}

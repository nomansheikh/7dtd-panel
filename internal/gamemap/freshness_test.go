package gamemap

import (
	"context"
	"testing"
	"time"

	"github.com/nomansheikh/7dtd-panel/internal/sdtd"
)

// How long a square stays trusted. A drawn tile and a square that has never
// been drawn have different answers, because only one of them can change while
// the world is empty.

// An admin filling in the map with visitmap redraws the world with nobody
// online, so "nobody is playing" cannot be taken to mean "nothing can appear".
// Holding a miss for the idle lifetime leaves the map blank long after the
// server has drawn it.
func TestASquareThatGetsDrawnShowsUpEvenWithNobodyPlaying(t *testing.T) {
	src := &fakeSource{err: sdtd.ErrNoTile}
	cache, clk := newCache(t, src, func(o *Options) {
		o.Busy = func() bool { return false }
		o.IdleTTL = 30 * time.Minute
		o.MissTTL = 15 * time.Second
	})

	key := Key{Z: 1, X: 1, Y: -1}
	tile, err := cache.Get(context.Background(), key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !tile.Missing {
		t.Fatal("want it reported missing to begin with")
	}

	// The operator runs visitmap. The server draws it; nobody has joined.
	src.set([]byte("terrain"), nil)
	clk.add(20 * time.Second)

	tile, err = cache.Get(context.Background(), key)
	if err != nil {
		t.Fatalf("Get after the square was drawn: %v", err)
	}
	if tile.Missing || string(tile.Bytes) != "terrain" {
		t.Fatalf("still reporting the square as blank %+v; the map would stay "+
			"empty for the whole idle lifetime after being drawn", tile)
	}
}

// The saving that justifies the long idle lifetime is for squares that exist:
// an empty world will not redraw one.
func TestADrawnTileIsStillHeldWhileTheWorldIsEmpty(t *testing.T) {
	src := &fakeSource{body: []byte("terrain")}
	cache, clk := newCache(t, src, func(o *Options) {
		o.Busy = func() bool { return false }
		o.IdleTTL = 30 * time.Minute
		o.MissTTL = 15 * time.Second
	})

	key := Key{Z: 1, X: 0, Y: 0}
	if _, err := cache.Get(context.Background(), key); err != nil {
		t.Fatalf("Get: %v", err)
	}
	clk.add(10 * time.Minute)
	if _, err := cache.Get(context.Background(), key); err != nil {
		t.Fatalf("Get while idle: %v", err)
	}
	if src.count() != 1 {
		t.Fatalf("asked the game server %d times for a drawn tile on an empty "+
			"server, want 1", src.count())
	}
}

/*
Package gamemap caches the rendered map tiles a game server draws.

Tiles are the only thing the panel proxies in bulk: one screenful of map is a
few hundred separate images, and panning asks for a few hundred more. The game
server sends no ETag, no Last-Modified and no Cache-Control on them, and its own
map client copes by appending a timestamp to every URL, which throws caching
away entirely. So the panel has to own it: this cache decides how long a tile
may be reused, gives each one an ETag derived from its bytes, and remembers the
squares the renderer has never drawn so an unexplored world does not mean
thousands of pointless requests upstream.
*/
package gamemap

import (
	"container/list"
	"context"
	"sync"
	"time"
)

// Key identifies one square. Zoom is the level; X and Y are in the game's own
// tile numbering.
type Key struct {
	Z, X, Y int
}

// Tile is one cached square. Missing means the renderer has never drawn it,
// which is the ordinary state of most of a world.
type Tile struct {
	Bytes   []byte
	ETag    string
	Missing bool
	// MaxAge is how long the browser may reuse it without asking again.
	MaxAge time.Duration
}

// Source fetches a tile from a game server.
type Source interface {
	MapTile(ctx context.Context, z, x, y int) ([]byte, error)
}

// Options configure a Cache.
type Options struct {
	Source Source
	// Busy reports whether anybody is in the world. Tiles can only change
	// while a player is loading chunks, so an empty server's map is frozen and
	// may be trusted for much longer. Nil means always busy.
	Busy func() bool
	// MaxBytes caps the cache. Zero selects defaultMaxBytes.
	MaxBytes int64
	// BusyTTL and IdleTTL override how long a drawn tile is trusted. Zero
	// selects the defaults.
	BusyTTL, IdleTTL time.Duration
	// MissTTL overrides how long "never drawn" is trusted. Zero selects the
	// default.
	MissTTL time.Duration
	Now     func() time.Time
}

const (
	// defaultMaxBytes holds roughly six thousand tiles at the ten to fifteen
	// kilobytes one usually weighs, which is a whole 6k world at full zoom.
	defaultMaxBytes = 64 << 20
	// defaultBusyTTL is short because a player exploring draws new ground as
	// they go, and an operator watching them move wants to see it appear. It
	// also has to be shorter than the map's own refresh interval, or the
	// browser answers that refresh out of its own cache and nothing new is
	// ever shown.
	defaultBusyTTL = 10 * time.Second
	// defaultIdleTTL is long because a tile that has been drawn will not be
	// redrawn by an empty world. This is the case that matters: a panel left
	// open on an idle server should cost nothing at all.
	defaultIdleTTL = 30 * time.Minute
	// defaultMissTTL is short, and deliberately not subject to the idle rule.
	//
	// "Nobody is playing" does not mean the map cannot change: an admin can
	// run visitmap and redraw the whole world with the server empty, which is
	// exactly how an operator fills in a map for the first time. Holding a
	// miss for half an hour after that leaves the map blank long after it has
	// been drawn, and makes the page look broken. A miss is also the cheapest
	// thing the game server answers, so re-asking often costs almost nothing.
	defaultMissTTL = 15 * time.Second
)

// Cache is a byte-capped LRU of tiles, safe for concurrent use.
type Cache struct {
	src      Source
	busy     func() bool
	maxBytes int64
	busyTTL  time.Duration
	idleTTL  time.Duration
	missTTL  time.Duration
	now      func() time.Time

	mu sync.Mutex
	blankState
	items    map[Key]*list.Element
	order    *list.List
	bytes    int64
	inflight map[Key]*fetch
}

// New builds a Cache.
func New(opts Options) *Cache {
	c := &Cache{
		src:      opts.Source,
		busy:     opts.Busy,
		maxBytes: opts.MaxBytes,
		busyTTL:  opts.BusyTTL,
		idleTTL:  opts.IdleTTL,
		missTTL:  opts.MissTTL,
		now:      opts.Now,
		items:    make(map[Key]*list.Element),
		order:    list.New(),
		inflight: make(map[Key]*fetch),
	}
	if c.maxBytes <= 0 {
		c.maxBytes = defaultMaxBytes
	}
	if c.busyTTL <= 0 {
		c.busyTTL = defaultBusyTTL
	}
	if c.idleTTL <= 0 {
		c.idleTTL = defaultIdleTTL
	}
	if c.missTTL <= 0 {
		c.missTTL = defaultMissTTL
	}
	if c.now == nil {
		c.now = time.Now
	}
	return c
}

// TTL is how long a tile fetched now may be trusted.
//
// A square the renderer has never drawn gets its own short life, because
// whether it exists can change without anybody being in the world.
func (c *Cache) TTL(tile Tile) time.Duration {
	if tile.Missing {
		return c.missTTL
	}
	if c.busy == nil || c.busy() {
		return c.busyTTL
	}
	return c.idleTTL
}

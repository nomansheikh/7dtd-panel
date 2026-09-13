package gamemap

import (
	"context"
	"errors"
	"sync"

	"github.com/nomansheikh/7dtd-panel/internal/sdtd"
)

// fetch is one upstream request that other callers may wait on.
type fetch struct {
	done sync.WaitGroup
	tile Tile
	err  error
}

/*
Get returns one tile, from the cache when it is fresh and from the game server
otherwise.

Concurrent requests for the same square share a single upstream fetch. Leaflet
asks for a tile again whenever it re-enters the viewport, two browser tabs ask
for the same squares, and a redraw asks for everything at once, so without this
one pan could mean the same image pulled several times over.

A square the renderer has never drawn comes back as a Tile with Missing set
rather than an error, and is cached like any other: on a world nobody has
explored that is nearly every square, and re-asking upstream for each one on
every pan is the difference between a map that opens and one that hangs.
*/
func (c *Cache) Get(ctx context.Context, key Key) (Tile, error) {
	if tile, ok := c.lookup(key); ok {
		return tile, nil
	}

	c.mu.Lock()
	if pending, ok := c.inflight[key]; ok {
		c.mu.Unlock()
		pending.done.Wait()
		return pending.tile, pending.err
	}
	pending := &fetch{}
	pending.done.Add(1)
	c.inflight[key] = pending
	c.mu.Unlock()

	tile, err := c.load(ctx, key)

	c.mu.Lock()
	delete(c.inflight, key)
	c.mu.Unlock()

	pending.tile, pending.err = tile, err
	pending.done.Done()
	return tile, err
}

// load fetches one tile from the game server and caches the result.
func (c *Cache) load(ctx context.Context, key Key) (Tile, error) {
	body, err := c.src.MapTile(ctx, key.Z, key.X, key.Y)

	switch {
	case errors.Is(err, sdtd.ErrNoTile):
		return c.store(key, Tile{Missing: true, ETag: missingETag}), nil
	case err != nil:
		// Hold the last known good square rather than showing a hole. A game
		// server that blinks should leave the map as it was, not erase it.
		if stale, ok := c.stale(key); ok {
			return stale, nil
		}
		return Tile{}, err
	}

	// A server with rendering on sends a placeholder rather than a 404 for
	// ground it has never drawn, so the status code alone does not say whether
	// there is anything there.
	if c.isBlank(ctx, body) {
		return c.store(key, Tile{Missing: true, ETag: missingETag}), nil
	}

	return c.store(key, Tile{Bytes: body, ETag: etag(body)}), nil
}

// stale returns a cached tile past its TTL, for use when the game server
// cannot be reached. The TTL says when to re-ask, not when to stop believing.
func (c *Cache) stale(key Key) (Tile, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	el, ok := c.items[key]
	if !ok {
		return Tile{}, false
	}
	tile := el.Value.(*entry).tile
	// Short, because this tile is already known to be behind: the browser
	// should come back and ask once the server is answering again.
	tile.MaxAge = c.busyTTL
	return tile, true
}

// Stats reports what the cache is holding, for the map page's own diagnostics.
func (c *Cache) Stats() (tiles int, bytes int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.items), c.bytes
}

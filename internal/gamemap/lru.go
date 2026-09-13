package gamemap

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// The storage half of the cache: what is held, in what order, and how a tile
// is identified once it is. The deciding half — how long a tile stays good and
// when to go back to the game server — is in cache.go.

type entry struct {
	key       Key
	tile      Tile
	fetchedAt time.Time
}

// lookup returns a cached tile if it is still fresh.
func (c *Cache) lookup(key Key) (Tile, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	el, ok := c.items[key]
	if !ok {
		return Tile{}, false
	}
	got := el.Value.(*entry)
	ttl := c.TTL(got.tile)
	if c.now().Sub(got.fetchedAt) >= ttl {
		return Tile{}, false
	}
	c.order.MoveToFront(el)

	tile := got.tile
	tile.MaxAge = ttl
	return tile, true
}

// store puts a tile in the cache, evicting from the back until it fits.
//
// When the refetched bytes are identical to what was already held the original
// ETag is kept, so a browser that revalidates still gets its 304 and the tile
// it already has stays good.
func (c *Cache) store(key Key, tile Tile) Tile {
	ttl := c.TTL(tile)
	tile.MaxAge = ttl

	c.mu.Lock()
	defer c.mu.Unlock()

	if el, ok := c.items[key]; ok {
		old := el.Value.(*entry)
		if old.tile.ETag == tile.ETag {
			old.fetchedAt = c.now()
			c.order.MoveToFront(el)
			kept := old.tile
			kept.MaxAge = ttl
			return kept
		}
		c.bytes -= int64(len(old.tile.Bytes))
		c.order.Remove(el)
		delete(c.items, key)
	}

	c.items[key] = c.order.PushFront(&entry{key: key, tile: tile, fetchedAt: c.now()})
	c.bytes += int64(len(tile.Bytes))

	for c.bytes > c.maxBytes {
		oldest := c.order.Back()
		if oldest == nil {
			break
		}
		dropped := oldest.Value.(*entry)
		c.bytes -= int64(len(dropped.tile.Bytes))
		c.order.Remove(oldest)
		delete(c.items, dropped.key)
	}
	return tile
}

// etag is a strong validator derived from the bytes themselves, because the
// game server offers nothing to derive one from.
func etag(b []byte) string {
	sum := sha256.Sum256(b)
	return `"` + hex.EncodeToString(sum[:16]) + `"`
}

// missingETag marks a square the renderer has never drawn. Constant, so a
// browser holding a 404 keeps revalidating it cheaply rather than refetching.
const missingETag = `"unrendered"`

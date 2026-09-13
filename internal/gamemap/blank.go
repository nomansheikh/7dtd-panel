package gamemap

import (
	"context"
	"sync"
)

/*
Telling "never drawn" from "drawn, and empty".

A server with rendering switched off answers 404 for a square it has no data
for, which is unambiguous. A server with rendering switched ON does not: it
answers 200 with a small placeholder image instead. On the server this was
built against that placeholder is 367 bytes and byte-for-byte identical for
every coordinate, including ones no world contains.

Taking those at face value means the panel believes an untouched world is fully
drawn: the "nothing has been rendered yet" explanation can never appear, and
every blank square is cached and re-served as though it were terrain.

Rather than hardcode the placeholder — a game update would change it and the
check would quietly stop working — the panel asks the server for a square that
cannot exist and remembers whatever comes back. Anything matching that is a
square the renderer has not drawn.
*/

// blankProbe is a tile coordinate no world reaches. The largest map the game
// generates is 16384 blocks, which is 128 tiles across at full zoom.
const blankProbe = 1 << 20

// learnBlank fetches the server's placeholder once and remembers its ETag.
//
// A server that answers 404 instead — which is what one with rendering off
// does — leaves this empty, and nothing downstream changes.
func (c *Cache) learnBlank(ctx context.Context) {
	c.blankOnce.Do(func() {
		body, err := c.src.MapTile(ctx, 0, blankProbe, blankProbe)
		if err != nil || len(body) == 0 {
			return
		}
		c.mu.Lock()
		c.blankETag = etag(body)
		c.mu.Unlock()
	})
}

// isBlank reports whether these bytes are the server's "no data here" image.
func (c *Cache) isBlank(ctx context.Context, body []byte) bool {
	c.learnBlank(ctx)

	c.mu.Lock()
	known := c.blankETag
	c.mu.Unlock()

	return known != "" && etag(body) == known
}

// blankState is the part of a Cache that learns the placeholder. Kept as its
// own type so the probe runs once per game server rather than once per process.
type blankState struct {
	blankOnce sync.Once
	blankETag string
}

package gamemap

import (
	"context"
	"sync/atomic"
	"testing"
)

// placeholderSource answers like a server with rendering switched on: a 200
// carrying a fixed placeholder image for anything it has not drawn.
type placeholderSource struct {
	drawn map[Key][]byte
	calls int32
}

func (p *placeholderSource) MapTile(_ context.Context, z, x, y int) ([]byte, error) {
	atomic.AddInt32(&p.calls, 1)
	if body, ok := p.drawn[Key{Z: z, X: x, Y: y}]; ok {
		return body, nil
	}
	return []byte("the 367-byte placeholder"), nil
}

// Without this the panel believes an untouched world is fully drawn, because
// every square answers 200.
func TestThePlaceholderAServerSendsForUndrawnGroundIsNotMistakenForTerrain(t *testing.T) {
	drawn := Key{Z: 1, X: 0, Y: 0}
	src := &placeholderSource{drawn: map[Key][]byte{drawn: []byte("real terrain")}}
	cache, _ := newCache(t, src, nil)

	blank, err := cache.Get(context.Background(), Key{Z: 1, X: 5, Y: 5})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !blank.Missing {
		t.Fatalf("the placeholder was taken for terrain: %d bytes", len(blank.Bytes))
	}

	real, err := cache.Get(context.Background(), drawn)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if real.Missing || string(real.Bytes) != "real terrain" {
		t.Fatalf("drawn ground was reported missing: %+v", real)
	}
}

// The probe costs one request for the life of the cache, not one per tile.
func TestTheServerIsAskedWhatBlankLooksLikeOnlyOnce(t *testing.T) {
	src := &placeholderSource{drawn: map[Key][]byte{}}
	cache, _ := newCache(t, src, nil)

	for x := range 6 {
		if _, err := cache.Get(context.Background(), Key{Z: 2, X: x, Y: 0}); err != nil {
			t.Fatalf("Get: %v", err)
		}
	}
	// Six tiles plus one probe.
	if got := int(atomic.LoadInt32(&src.calls)); got != 7 {
		t.Fatalf("made %d requests for 6 tiles, want 7 (one probe)", got)
	}
}

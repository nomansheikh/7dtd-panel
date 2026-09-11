package httpx

import (
	"crypto/rand"
	"encoding/hex"
)

// newID returns a short random identifier for request correlation. It is not
// security-sensitive, so collisions only cost log clarity.
func newID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(b[:])
}

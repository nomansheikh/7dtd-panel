// Package events fans the game server's log out to browser clients.
//
// Exactly one connection to the game server's SSE stream exists, regardless of
// how many browser tabs are open. Opening that stream makes the server write a
// log line announcing it, so a connection per tab would make the feed noisier
// the more people were watching it.
//
// A ring buffer of recent events means a newly opened tab gets immediate
// scrollback without another round trip to the game server.
package events

import (
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/nomansheikh/7dtd-panel/internal/sdtd"
)

// Kind classifies an event for filtering in the UI.
type Kind string

const (
	// KindLog is an ordinary server log line.
	KindLog Kind = "log"
	// KindChat is a log line identified as in-game chat.
	KindChat Kind = "chat"
	// KindJoin and KindLeave mark players entering and leaving.
	KindJoin  Kind = "join"
	KindLeave Kind = "leave"
	// KindDeath is a player dying, which the game also announces over GMSG.
	KindDeath Kind = "death"
	// KindStatus is emitted by the panel itself, not the game server.
	KindStatus Kind = "status"
)

// Event is one item in the feed.
type Event struct {
	// Seq is assigned by the panel and always increases, so a client can tell
	// whether it missed anything. The game server's own log id is in LogID.
	Seq  int64     `json:"seq"`
	Kind Kind      `json:"kind"`
	At   time.Time `json:"at"`

	// LogID is the game server's log line number, absent for panel events.
	LogID *int `json:"logId,omitempty"`
	// Severity mirrors the server's log type: Log, Warning, Error, Exception.
	Severity string `json:"severity,omitempty"`
	Message  string `json:"message"`

	// Player is set for chat, join and leave events when it could be parsed.
	Player string `json:"player,omitempty"`
	// Channel is the chat channel, set only when it is not Global. Party chat
	// reads very differently from something said to the whole server.
	Channel string `json:"channel,omitempty"`
	// Raw is the server's original log line, kept only when Message is a
	// tidied version of it. Nothing the server said is thrown away; it is just
	// not what the eye lands on first.
	Raw string `json:"raw,omitempty"`
}

// ringSize is how much scrollback a newly connected tab receives. Two thousand
// lines is a few minutes of a busy server and costs well under a megabyte.
const ringSize = 2000

// subscriberBuffer bounds how far behind a single client may fall before it is
// dropped, so one stalled tab cannot apply backpressure to the whole hub.
const subscriberBuffer = 256

// Hub broadcasts events to subscribers and retains recent history.
type Hub struct {
	mu     sync.RWMutex
	ring   []Event
	nextID int64
	subs   map[int64]chan Event
	nextNo int64
	// dropped counts subscribers disconnected for falling behind.
	dropped int64
}

// NewHub builds an empty hub.
func NewHub() *Hub {
	return &Hub{
		ring: make([]Event, 0, ringSize),
		subs: make(map[int64]chan Event),
	}
}

// PublishLog converts a game server log entry into an event and broadcasts it.
func (h *Hub) PublishLog(entry sdtd.LogEntry) {
	at := time.Now().UTC()
	if t, ok := entry.Time(); ok {
		at = t.UTC()
	}
	id := entry.ID

	c := classify(entry.Msg)

	// The raw line is kept only when it differs, so the common case — an
	// ordinary log line nobody reformatted — costs nothing extra on the wire.
	raw := ""
	if c.Text != entry.Msg {
		raw = entry.Msg
	}

	h.Publish(Event{
		Kind:     c.Kind,
		At:       at,
		LogID:    &id,
		Severity: entry.Type,
		Message:  c.Text,
		Player:   c.Player,
		Channel:  c.Channel,
		Raw:      raw,
	})
}

// PublishStatus broadcasts a panel-generated notice, such as the game server
// becoming unreachable.
func (h *Hub) PublishStatus(message string) {
	h.Publish(Event{Kind: KindStatus, At: time.Now().UTC(), Message: message})
}

// Publish assigns a sequence number, records the event and broadcasts it.
func (h *Hub) Publish(e Event) {
	h.mu.Lock()
	h.nextID++
	e.Seq = h.nextID
	if e.At.IsZero() {
		e.At = time.Now().UTC()
	}

	if len(h.ring) == ringSize {
		copy(h.ring, h.ring[1:])
		h.ring[len(h.ring)-1] = e
	} else {
		h.ring = append(h.ring, e)
	}

	for id, ch := range h.subs {
		select {
		case ch <- e:
		default:
			// Full buffer means this client has stopped reading. Drop it rather
			// than block everyone else; its browser will reconnect.
			close(ch)
			delete(h.subs, id)
			h.dropped++
		}
	}
	h.mu.Unlock()
}

// Subscribe returns the current backlog and a channel of subsequent events.
//
// The backlog is captured under the same lock that registers the channel, so
// nothing can slip between the two and be missed or duplicated.
func (h *Hub) Subscribe(backlog int) (history []Event, ch <-chan Event, cancel func()) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if backlog > len(h.ring) || backlog < 0 {
		backlog = len(h.ring)
	}
	history = make([]Event, backlog)
	copy(history, h.ring[len(h.ring)-backlog:])

	h.nextNo++
	id := h.nextNo
	out := make(chan Event, subscriberBuffer)
	h.subs[id] = out

	return history, out, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if existing, ok := h.subs[id]; ok {
			close(existing)
			delete(h.subs, id)
		}
	}
}

// Stats reports hub state, for diagnostics and tests.
func (h *Hub) Stats() (subscribers, buffered int, dropped int64) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.subs), len(h.ring), h.dropped
}

// Chat and connection notices arrive as ordinary log lines; the server has no
// separate event type for them, and no SSE event name other than "log" exists.
//
// Verified against a live server with a player online. The real lines are:
//
//	Chat (from 'Steam_7656119...', entity id '173', to 'Global'): 'nullish': hello
//	GMSG: Player 'nullish' joined the game
//	GMSG: Player 'nullish' died
//
// The leave variant is inferred from the same GMSG shape and has not yet been
// seen on a live server.
//
// They remain written to fail safe: anything unrecognised stays an ordinary log
// entry rather than being mislabelled.
var (
	// The speaker's name is optional: the server's own broadcasts, which is
	// what the say command produces, omit it entirely. Verified live, both
	// shapes:
	//
	//	Chat (from 'Steam_76561198803325430', entity id '173', to 'Global'): 'nullish': lkj
	//	Chat (from '-non-player-', entity id '-1', to 'Global'): restarting in 5
	chatRe  = regexp.MustCompile(`^Chat\s*\(from\s*'([^']*)',\s*entity id\s*'(-?\d+)',\s*to\s*'([^']*)'\):\s*(?:'([^']*)':\s*)?(.*)$`)
	joinRe  = regexp.MustCompile(`^GMSG:\s*Player\s*'([^']*)'\s+joined`)
	partRe  = regexp.MustCompile(`^GMSG:\s*Player\s*'([^']*)'\s+left`)
	deathRe = regexp.MustCompile(`^GMSG:\s*Player\s*'([^']*)'\s+died`)
)

// classified is what a log line turned out to be.
type classified struct {
	Kind    Kind
	Player  string
	Channel string
	// Text is the part worth reading. For a chat line that is what was
	// actually said, not the forty characters of platform id wrapped around
	// it; for anything unrecognised it is the line unchanged.
	Text string
}

// classify labels a log message, extracts the player where it can, and reduces
// the line to the part a person wants to read.
//
// The server says:
//
//	Chat (from 'Steam_76561198803325430', entity id '173', to 'Global'): 'nullish': hello
//
// A panel that shows that verbatim has made the operator do the parsing. The
// name is already its own column and the platform id is on the players page,
// so what is left is "hello".
func classify(msg string) classified {
	trimmed := strings.TrimSpace(msg)

	if m := chatRe.FindStringSubmatch(trimmed); m != nil {
		channel := m[3]
		if strings.EqualFold(channel, "Global") {
			channel = ""
		}
		// A line with no speaker came from the server itself, which is what an
		// operator sees after using say. Attributing it to "Server" is what
		// the game shows players, so it is what the panel should show too.
		speaker := m[4]
		if speaker == "" {
			speaker = "Server"
		}
		return classified{Kind: KindChat, Player: speaker, Channel: channel, Text: m[5]}
	}
	if m := joinRe.FindStringSubmatch(trimmed); m != nil {
		return classified{Kind: KindJoin, Player: m[1], Text: "joined the game"}
	}
	if m := partRe.FindStringSubmatch(trimmed); m != nil {
		return classified{Kind: KindLeave, Player: m[1], Text: "left the game"}
	}
	if m := deathRe.FindStringSubmatch(trimmed); m != nil {
		return classified{Kind: KindDeath, Player: m[1], Text: "died"}
	}
	return classified{Kind: KindLog, Text: msg}
}

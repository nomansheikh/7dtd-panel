package events

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nomansheikh/7dtd-panel/internal/sdtd"
)

func TestPublishAssignsIncreasingSequence(t *testing.T) {
	h := NewHub()
	for i := 0; i < 5; i++ {
		h.PublishStatus("tick")
	}
	history, _, cancel := h.Subscribe(-1)
	defer cancel()

	if len(history) != 5 {
		t.Fatalf("history = %d events, want 5", len(history))
	}
	for i := 1; i < len(history); i++ {
		if history[i].Seq <= history[i-1].Seq {
			t.Errorf("sequence did not increase at %d: %d then %d",
				i, history[i-1].Seq, history[i].Seq)
		}
	}
}

func TestSubscriberReceivesSubsequentEvents(t *testing.T) {
	h := NewHub()
	h.PublishStatus("before")

	history, ch, cancel := h.Subscribe(-1)
	defer cancel()

	if len(history) != 1 || history[0].Message != "before" {
		t.Fatalf("backlog = %+v, want the one earlier event", history)
	}

	h.PublishStatus("after")
	select {
	case got := <-ch:
		if got.Message != "after" {
			t.Errorf("message = %q, want after", got.Message)
		}
	case <-time.After(time.Second):
		t.Fatal("subscriber never received the event")
	}
}

// TestSubscribeIsAtomic guards the window between reading history and
// registering the channel: an event published in between must not vanish.
func TestSubscribeIsAtomic(t *testing.T) {
	h := NewHub()
	var wg sync.WaitGroup

	// Hammer the hub while subscribing repeatedly.
	stop := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				h.PublishStatus("noise")
			}
		}
	}()

	for i := 0; i < 50; i++ {
		history, ch, cancel := h.Subscribe(10)
		var lastHistory int64
		if len(history) > 0 {
			lastHistory = history[len(history)-1].Seq
		}
		select {
		case got := <-ch:
			// The first live event must come immediately after the backlog,
			// with nothing skipped.
			if lastHistory != 0 && got.Seq != lastHistory+1 {
				t.Fatalf("gap: backlog ended at %d, first live event was %d",
					lastHistory, got.Seq)
			}
		case <-time.After(time.Second):
		}
		cancel()
	}
	close(stop)
	wg.Wait()
}

func TestRingBufferIsBounded(t *testing.T) {
	h := NewHub()
	for i := 0; i < ringSize+100; i++ {
		h.PublishStatus("filler")
	}
	history, _, cancel := h.Subscribe(-1)
	defer cancel()

	if len(history) != ringSize {
		t.Errorf("history = %d, want it capped at %d", len(history), ringSize)
	}
	// The oldest entries should have been evicted, leaving the newest.
	if history[len(history)-1].Seq != int64(ringSize+100) {
		t.Errorf("newest seq = %d, want %d", history[len(history)-1].Seq, ringSize+100)
	}
	if history[0].Seq != 101 {
		t.Errorf("oldest retained seq = %d, want 101", history[0].Seq)
	}
}

func TestBacklogLimit(t *testing.T) {
	h := NewHub()
	for i := 0; i < 20; i++ {
		h.PublishStatus("x")
	}
	history, _, cancel := h.Subscribe(5)
	defer cancel()

	if len(history) != 5 {
		t.Fatalf("history = %d, want 5", len(history))
	}
	// Asking for 5 should give the five most recent, not the five oldest.
	if history[len(history)-1].Seq != 20 {
		t.Errorf("newest seq = %d, want 20", history[len(history)-1].Seq)
	}
}

// TestSlowSubscriberIsDroppedNotBlocking is the property that keeps one stalled
// browser tab from stalling everyone else.
func TestSlowSubscriberIsDroppedNotBlocking(t *testing.T) {
	h := NewHub()

	_, slow, cancelSlow := h.Subscribe(0)
	defer cancelSlow()
	_, fast, cancelFast := h.Subscribe(0)
	defer cancelFast()

	// Never read from slow. Publish far more than its buffer holds.
	done := make(chan struct{})
	go func() {
		for i := 0; i < subscriberBuffer*3; i++ {
			h.PublishStatus("flood")
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Publish blocked on a subscriber that stopped reading")
	}

	// The slow one should have been closed.
	drained := 0
	for range slow {
		drained++
	}
	if drained == 0 {
		t.Error("the slow subscriber received nothing at all")
	}

	_, _, dropped := h.Stats()
	if dropped == 0 {
		t.Error("no subscriber was recorded as dropped")
	}

	// The fast one must still be usable.
	go func() {
		for range fast {
		}
	}()
}

func TestPublishLogCarriesServerFields(t *testing.T) {
	h := NewHub()
	h.PublishLog(sdtd.LogEntry{
		ID:      42,
		Msg:     "StartGame done",
		Type:    "Warning",
		ISOTime: "2026-09-11T08:00:00.0000000+00:00",
		Uptime:  "1234",
	})

	history, _, cancel := h.Subscribe(-1)
	defer cancel()

	got := history[0]
	if got.LogID == nil || *got.LogID != 42 {
		t.Errorf("logId = %v, want 42", got.LogID)
	}
	if got.Severity != "Warning" {
		t.Errorf("severity = %q, want Warning", got.Severity)
	}
	if got.Kind != KindLog {
		t.Errorf("kind = %q, want log", got.Kind)
	}
	// The server's own timestamp should be preferred over arrival time.
	if got.At.Year() != 2026 || got.At.Month() != time.September {
		t.Errorf("at = %s, want the server's isotime", got.At)
	}
}

func TestPublishLogFallsBackWhenTimestampIsUnparseable(t *testing.T) {
	h := NewHub()
	h.PublishLog(sdtd.LogEntry{ID: 1, Msg: "x", ISOTime: "not a time"})
	history, _, cancel := h.Subscribe(-1)
	defer cancel()

	if history[0].At.IsZero() {
		t.Error("a bad server timestamp should fall back to arrival time, not zero")
	}
}

// Tidying the message must not lose what the server actually said: the raw
// line is what an operator falls back to when the parse looks wrong.
func TestPublishLogKeepsTheRawLineOnlyWhenItWasChanged(t *testing.T) {
	h := NewHub()
	_, ch, done := h.Subscribe(0)
	defer done()

	h.PublishLog(sdtd.LogEntry{
		ID:   1,
		Type: "Log",
		Msg:  `Chat (from 'Steam_1', entity id '3', to 'Global'): 'Bob': hello`,
	})
	chat := <-ch
	if chat.Message != "hello" {
		t.Errorf("message = %q, want the text that was said", chat.Message)
	}
	if !strings.Contains(chat.Raw, "Steam_1") {
		t.Errorf("raw = %q, want the server's original line", chat.Raw)
	}

	h.PublishLog(sdtd.LogEntry{ID: 2, Type: "Log", Msg: "StartGame done"})
	plain := <-ch
	if plain.Message != "StartGame done" {
		t.Errorf("message = %q", plain.Message)
	}
	if plain.Raw != "" {
		t.Errorf("raw = %q, want empty when nothing was rewritten", plain.Raw)
	}
}

func TestClassify(t *testing.T) {
	// The first two cases are verbatim lines captured from a live server with a
	// player online, so the formats are confirmed rather than guessed. The
	// important property remains that anything unrecognised stays an ordinary
	// log entry.
	tests := []struct {
		name        string
		msg         string
		wantKind    Kind
		wantPlayer  string
		wantText    string
		wantChannel string
	}{
		{
			// Captured verbatim from a live server.
			name:       "global chat",
			msg:        `Chat (from 'Steam_76561198803325430', entity id '173', to 'Global'): 'nullish': hello there`,
			wantKind:   KindChat,
			wantPlayer: "nullish",
			// What was said, not the platform id wrapped around it.
			wantText: "hello there",
		},
		{
			name:       "chat to a party",
			msg:        `Chat (from 'Steam_1', entity id '3', to 'Party'): 'Bob': on my way`,
			wantKind:   KindChat,
			wantPlayer: "Bob",
			wantText:   "on my way",
			// Party chat is worth distinguishing; Global is the default and
			// carries no channel.
			wantChannel: "Party",
		},
		{
			name:       "server chat with an explicit name",
			msg:        `Chat (from 'Steam_-1', entity id '-1', to 'Global'): 'Server': restarting soon`,
			wantKind:   KindChat,
			wantPlayer: "Server",
			wantText:   "restarting soon",
		},
		{
			// Captured verbatim: this is what the say command produces, and it
			// omits the speaker entirely rather than naming the server.
			name:       "server broadcast names no speaker",
			msg:        `Chat (from '-non-player-', entity id '-1', to 'Global'): restarting in 5`,
			wantKind:   KindChat,
			wantPlayer: "Server",
			wantText:   "restarting in 5",
		},
		{
			// Captured verbatim from a live server.
			name:       "join",
			msg:        `GMSG: Player 'nullish' joined the game`,
			wantKind:   KindJoin,
			wantPlayer: "nullish",
			wantText:   "joined the game",
		},
		{
			// Captured verbatim from a live server.
			name:       "death",
			msg:        `GMSG: Player 'nullish' died`,
			wantKind:   KindDeath,
			wantPlayer: "nullish",
			wantText:   "died",
		},
		{
			// Inferred from the same GMSG shape; not yet seen on a live server.
			name:       "leave",
			msg:        `GMSG: Player 'Noman' left the game`,
			wantKind:   KindLeave,
			wantPlayer: "Noman",
			wantText:   "left the game",
		},
		{
			name:     "ordinary log line",
			msg:      "StartGame done",
			wantKind: KindLog,
			// Unrecognised lines come through untouched.
			wantText: "StartGame done",
		},
		{
			name:     "a command echo is not chat",
			msg:      "Executing command 'gettime' by WebCommandResult_for_gettime_by_Unauth-PermLevel-0",
			wantKind: KindLog,
			wantText: "Executing command 'gettime' by WebCommandResult_for_gettime_by_Unauth-PermLevel-0",
		},
		{
			name:     "something merely mentioning chat is not chat",
			msg:      "INF Chat system initialised",
			wantKind: KindLog,
			wantText: "INF Chat system initialised",
		},
		{
			name:     "empty",
			msg:      "",
			wantKind: KindLog,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classify(tt.msg)
			if got.Kind != tt.wantKind {
				t.Errorf("kind = %q, want %q", got.Kind, tt.wantKind)
			}
			if got.Player != tt.wantPlayer {
				t.Errorf("player = %q, want %q", got.Player, tt.wantPlayer)
			}
			if got.Text != tt.wantText {
				t.Errorf("text = %q, want %q", got.Text, tt.wantText)
			}
			if got.Channel != tt.wantChannel {
				t.Errorf("channel = %q, want %q", got.Channel, tt.wantChannel)
			}
		})
	}
}

func TestConcurrentPublishAndSubscribe(t *testing.T) {
	h := NewHub()
	var wg sync.WaitGroup

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				h.PublishStatus("concurrent")
			}
		}()
	}
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_, ch, cancel := h.Subscribe(5)
				go func() {
					for range ch {
					}
				}()
				cancel()
			}
		}()
	}
	wg.Wait()
}

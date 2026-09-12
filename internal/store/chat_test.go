package store

import (
	"context"
	"testing"
	"time"
)

// The cooldown is the only thing between one command and a player holding the
// key down, so the database decides rather than a read followed by a write.
func TestTakeCooldown(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()

	ok, _, err := s.TakeCooldown(ctx, "alpha", "Steam_1", "kit", time.Hour, now)
	if err != nil || !ok {
		t.Fatalf("first use = %v, %v; want allowed", ok, err)
	}

	ok, left, err := s.TakeCooldown(ctx, "alpha", "Steam_1", "kit", time.Hour, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("a second use inside the hour was allowed")
	}
	if left < 58*time.Minute || left > time.Hour {
		t.Errorf("time left = %v, want about 59 minutes", left)
	}

	// Somebody else is not on this player's cooldown.
	ok, _, err = s.TakeCooldown(ctx, "alpha", "Steam_2", "kit", time.Hour, now.Add(time.Minute))
	if err != nil || !ok {
		t.Errorf("another player = %v, %v; want allowed", ok, err)
	}

	// A different server is a different cooldown: one panel watching two
	// worlds must not make a kit on one cost a kit on the other.
	ok, _, err = s.TakeCooldown(ctx, "beta", "Steam_1", "kit", time.Hour, now.Add(time.Minute))
	if err != nil || !ok {
		t.Errorf("another server = %v, %v; want allowed", ok, err)
	}

	// And it lapses.
	ok, _, err = s.TakeCooldown(ctx, "alpha", "Steam_1", "kit", time.Hour, now.Add(2*time.Hour))
	if err != nil || !ok {
		t.Errorf("after the hour = %v, %v; want allowed", ok, err)
	}
}

// A command that failed gives the wait back.
func TestClearCooldown(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()

	if _, _, err := s.TakeCooldown(ctx, "alpha", "Steam_1", "kit", time.Hour, now); err != nil {
		t.Fatal(err)
	}
	if err := s.ClearCooldown(ctx, "alpha", "Steam_1", "kit"); err != nil {
		t.Fatal(err)
	}
	ok, _, err := s.TakeCooldown(ctx, "alpha", "Steam_1", "kit", time.Hour, now.Add(time.Second))
	if err != nil || !ok {
		t.Errorf("after clearing = %v, %v; want allowed", ok, err)
	}

	// Clearing one that was never taken is not an error.
	if err := s.ClearCooldown(ctx, "alpha", "Steam_9", "kit"); err != nil {
		t.Error(err)
	}
}

// No cooldown configured means no bookkeeping at all.
func TestTakeCooldownZeroIsAlwaysAllowed(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	now := time.Now()

	for i := 0; i < 3; i++ {
		ok, _, err := s.TakeCooldown(ctx, "alpha", "Steam_1", "day", 0, now)
		if err != nil || !ok {
			t.Fatalf("use %d = %v, %v; want allowed", i, ok, err)
		}
	}
}

func TestChatCommandsRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()

	if err := s.SaveChatCommand(ctx, "alpha", ChatCommand{
		Name: "kit", Enabled: true, Audience: AudienceAdmins, CooldownSeconds: 3600,
	}, now); err != nil {
		t.Fatal(err)
	}
	// Saving again is an update, not a second row.
	if err := s.SaveChatCommand(ctx, "alpha", ChatCommand{
		Name: "kit", Enabled: false, Audience: AudienceEveryone, CooldownSeconds: 60,
	}, now); err != nil {
		t.Fatal(err)
	}

	// A second server's row with the same name is a different command.
	if err := s.SaveChatCommand(ctx, "beta", ChatCommand{
		Name: "kit", Enabled: true, Audience: AudienceAdmins, CooldownSeconds: 30,
	}, now); err != nil {
		t.Fatal(err)
	}

	got, err := s.ChatCommands(ctx, "alpha")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("commands = %d, want 1", len(got))
	}
	if got[0].Enabled || got[0].Audience != AudienceEveryone || got[0].CooldownSeconds != 60 {
		t.Errorf("command = %+v", got[0])
	}
}

func TestKitsRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	now := time.Now()

	if err := s.SaveKit(ctx, "starter", `[{"item":"resourceWood","count":10,"quality":0}]`, now); err != nil {
		t.Fatal(err)
	}
	got, err := s.Kit(ctx, "starter")
	if err != nil {
		t.Fatal(err)
	}
	if got.Items == "" {
		t.Error("items came back empty")
	}

	if _, err := s.Kit(ctx, "nope"); err != ErrNotFound {
		t.Errorf("missing kit = %v, want ErrNotFound", err)
	}
	if err := s.DeleteKit(ctx, "starter"); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteKit(ctx, "starter"); err != ErrNotFound {
		t.Errorf("deleting twice = %v, want ErrNotFound", err)
	}
}

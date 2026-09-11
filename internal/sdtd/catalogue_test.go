package sdtd

import (
	"context"
	"strings"
	"testing"
)

// The buff list comes out of an error message, which is the only place the
// server publishes it. This is exactly the sort of parsing that breaks on a
// game update without anyone noticing, so it is checked against a real server.
func TestIntegrationBuffs(t *testing.T) {
	c := liveClient(t)
	buffs, err := c.Buffs(context.Background())
	if err != nil {
		t.Fatalf("Buffs: %v", err)
	}
	if len(buffs) < 100 {
		t.Fatalf("got %d buffs, expected the server's full list", len(buffs))
	}

	named := 0
	for _, b := range buffs {
		if b.Name == "" {
			t.Fatalf("a buff came back with no name: %+v", b)
		}
		if strings.ContainsAny(b.Name, " ()") {
			t.Errorf("name %q still has list punctuation in it", b.Name)
		}
		if b.LocalizedName != "" {
			named++
		}
	}
	if named == 0 {
		t.Error("no display names were parsed; the list format has changed")
	}
	t.Logf("%d buffs, %d with a display name, e.g. %+v", len(buffs), named, buffs[0])
}

// The sandbox list is what turns the settings page into pickers rather than
// text fields, so its shape matters more than its values.
func TestIntegrationSandboxSettings(t *testing.T) {
	c := liveClient(t)
	settings, err := c.SandboxSettings(context.Background())
	if err != nil {
		t.Fatalf("SandboxSettings: %v", err)
	}
	if settings.Code == "" {
		t.Error("no sandbox code")
	}
	if len(settings.Options) == 0 {
		t.Fatal("no sandbox options")
	}

	withChoices, withDetails := 0, 0
	for _, o := range settings.Options {
		if o.Key == "" {
			t.Fatalf("option with no key: %+v", o)
		}
		if len(o.ValueSet) > 0 {
			withChoices++
		}
		if o.Details != nil && o.Details.Description != "" {
			withDetails++
		}
	}
	if withChoices != len(settings.Options) {
		t.Errorf("%d of %d options enumerate their values; the page assumes all do",
			withChoices, len(settings.Options))
	}
	if withDetails == 0 {
		t.Error("no descriptions came back; detailed=true is not being sent")
	}
	t.Logf("%d options, %d described, e.g. %s = %s (%s)", len(settings.Options), withDetails,
		settings.Options[0].Key, settings.Options[0].ActiveValue.String(),
		settings.Options[0].ActiveValue.Label)
}

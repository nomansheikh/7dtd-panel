package console

import (
	"strings"
	"testing"
)

func TestBuildersProduceExpectedCommands(t *testing.T) {
	tests := []struct {
		name  string
		build func() (string, error)
		want  string
	}{
		{"teleport to coords", func() (string, error) { return Teleport(171, -123, 61, 734) },
			"teleportplayer 171 -123 61 734"},
		{"teleport to ground level", func() (string, error) { return Teleport(171, 10, -1, 20) },
			"teleportplayer 171 10 -1 20"},
		{"teleport to another player", func() (string, error) { return TeleportToPlayer(171, 172) },
			"teleportplayer 171 172"},
		{"give without quality", func() (string, error) { return GiveItem(171, "meleeToolStoneAxe", 1, 0) },
			"give 171 meleeToolStoneAxe 1"},
		{"give with quality", func() (string, error) { return GiveItem(171, "gunHandgunT1Pistol", 2, 6) },
			"give 171 gunHandgunT1Pistol 2 6"},
		{"kill", func() (string, error) { return Kill(171) }, "kill 171"},
		{"kick without reason", func() (string, error) { return Kick(171, "") }, "kick 171"},
		{"kick with reason", func() (string, error) { return Kick(171, "afk too long") },
			"kick 171 afk too long"},
		{"ban", func() (string, error) {
			return Ban("Steam_76561198021925107", 3, BanDays, "griefing")
		}, "ban add Steam_76561198021925107 3 days griefing"},
		{"unban", func() (string, error) { return Unban("Steam_76561198021925107") },
			"ban remove Steam_76561198021925107"},
		{"buff", func() (string, error) { return Buff(171, "buffBrokenLeg") },
			"buffplayer 171 buffBrokenLeg"},
		{"debuff", func() (string, error) { return Debuff(171, "buffBrokenLeg") },
			"debuffplayer 171 buffBrokenLeg"},
		{"give xp", func() (string, error) { return GiveXP(171, 5000) }, "givexp 171 5000"},
		{"set time", func() (string, error) { return SetTime(7, 21, 30) }, "settime 7 21 30"},
		{"weather rain", func() (string, error) { return Weather(WeatherRain, 0.5) }, "weather Rain 0.5"},
		{"weather integral value has no trailing zeros", func() (string, error) {
			return Weather(WeatherWind, 100)
		}, "weather Wind 100"},
		{"negative temperature", func() (string, error) { return Weather(WeatherTemp, -20) },
			"weather Temp -20"},
		{"spawn entity by name", func() (string, error) { return SpawnEntityAt("zombieArlene", 10, -1, 20, 3) },
			"spawnentityat zombieArlene 10 -1 20 3"},
		{"say", func() (string, error) { return Say("server restarting in 5 minutes") },
			"say server restarting in 5 minutes"},
		{"set game pref", func() (string, error) { return SetGamePref("EnableMapRendering", "true") },
			"setgamepref EnableMapRendering true"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.build()
			if err != nil {
				t.Fatalf("builder returned %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
			// Whatever a builder produces must survive the console's own check.
			if err := Validate(got); err != nil {
				t.Errorf("builder produced a command Validate rejects: %v", err)
			}
		})
	}
}

// TestBuildersRejectInjection is the point of this package. A player can name
// themselves almost anything, and item names arrive from a catalogue, so no
// argument may be able to terminate one command and begin another.
func TestBuildersRejectInjection(t *testing.T) {
	hostile := []string{
		"axe\nshutdown",
		"axe\rshutdown",
		"axe shutdown",
		"axe;shutdown",
		`axe"; shutdown`,
		"axe'",
		"../../etc/passwd",
		"axe\x00shutdown",
		"axe$(shutdown)",
		"axe`shutdown`",
		"axe|shutdown",
		"axe&&shutdown",
	}

	for _, payload := range hostile {
		t.Run(payload, func(t *testing.T) {
			if _, err := GiveItem(171, payload, 1, 0); err == nil {
				t.Errorf("GiveItem accepted %q", payload)
			}
			if _, err := Buff(171, payload); err == nil {
				t.Errorf("Buff accepted %q", payload)
			}
			if _, err := SetGamePref(payload, "1"); err == nil {
				t.Errorf("SetGamePref accepted %q as a name", payload)
			}
		})
	}
}

func TestFreeTextRejectsQuotesAndBreaks(t *testing.T) {
	// Reasons and messages are written by a person, so more is allowed, but a
	// line break or quote could still change how the game parses the command.
	tests := []struct {
		name    string
		text    string
		wantErr bool
	}{
		{"plain words", "being rude to others", false},
		{"punctuation is fine", "spawn camping, repeatedly!", false},
		{"newline", "rude\nshutdown", true},
		{"carriage return", "rude\rshutdown", true},
		{"double quote", `said "hello"`, true},
		{"single quote", "don't", true},
		{"null byte", "rude\x00", true},
		{"very long", strings.Repeat("x", 500), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Kick(171, tt.text)
			if tt.wantErr && err == nil {
				t.Errorf("Kick accepted %q", tt.text)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Kick rejected %q: %v", tt.text, err)
			}
		})
	}
}

func TestBuildersValidateRanges(t *testing.T) {
	tests := []struct {
		name  string
		build func() (string, error)
	}{
		{"negative entity id", func() (string, error) { return Kill(-1) }},
		{"zero item count", func() (string, error) { return GiveItem(1, "axe", 0, 0) }},
		{"absurd item count", func() (string, error) { return GiveItem(1, "axe", 999999, 0) }},
		{"quality above six", func() (string, error) { return GiveItem(1, "axe", 1, 7) }},
		{"day below one", func() (string, error) { return SetTime(0, 0, 0) }},
		{"hour above 23", func() (string, error) { return SetTime(1, 24, 0) }},
		{"minute above 59", func() (string, error) { return SetTime(1, 0, 60) }},
		{"rain above one", func() (string, error) { return Weather(WeatherRain, 1.5) }},
		{"wind above 200", func() (string, error) { return Weather(WeatherWind, 500) }},
		{"temperature below -99", func() (string, error) { return Weather(WeatherTemp, -200) }},
		{"unknown weather knob", func() (string, error) { return Weather(WeatherKnob("Sunshine"), 1) }},
		{"zero xp", func() (string, error) { return GiveXP(1, 0) }},
		{"spawn count too high", func() (string, error) { return SpawnEntityAt("zombieArlene", 0, 0, 0, 1000) }},
		{"spawn with an injected class name", func() (string, error) { return SpawnEntityAt("zombie shutdown", 0, 0, 0, 1) }},
		{"teleport to self", func() (string, error) { return TeleportToPlayer(171, 171) }},
		{"empty say", func() (string, error) { return Say("   ") }},
		{"bad ban id", func() (string, error) { return Ban("Noman", 1, BanDays, "") }},
		{"bad ban unit", func() (string, error) { return Ban("Steam_1", 1, BanUnit("fortnights"), "") }},
		{"zero ban duration", func() (string, error) { return Ban("Steam_1", 0, BanDays, "") }},
		{"empty pref value", func() (string, error) { return SetGamePref("Name", "") }},
		{"pref value with a space", func() (string, error) { return SetGamePref("Name", "two words") }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := tt.build(); err == nil {
				t.Error("builder accepted an out-of-range argument")
			}
		})
	}
}

func TestBanAcceptsEveryUnitTheServerDocuments(t *testing.T) {
	for _, unit := range []BanUnit{BanMinutes, BanHours, BanDays, BanWeeks, BanMonths, BanYears} {
		t.Run(string(unit), func(t *testing.T) {
			got, err := Ban("Steam_76561198021925107", 1, unit, "")
			if err != nil {
				t.Fatalf("Ban rejected %q: %v", unit, err)
			}
			if !strings.HasSuffix(got, string(unit)) {
				t.Errorf("got %q, want it to end with %q", got, unit)
			}
		})
	}
}

func TestHordeBuildersAreNotBloodMoon(t *testing.T) {
	// /api/bloodmoon is read-only and there is no bloodmoon command, so this
	// must not pretend otherwise.
	if got := SpawnWanderingHorde(); got != "spawnwandering" {
		t.Errorf("got %q", got)
	}
	got, err := SpawnScouts(171)
	if err != nil {
		t.Fatalf("SpawnScouts: %v", err)
	}
	if got != "spawnscouts 171" {
		t.Errorf("got %q", got)
	}
}

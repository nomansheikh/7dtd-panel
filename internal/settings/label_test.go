package settings

import "testing"

func TestLabel(t *testing.T) {
	for _, tt := range []struct{ in, want string }{
		{"AirDropFrequency", "Air drop frequency"},
		{"BloodMoonEnemyCount", "Blood moon enemy count"},
		{"XPMultiplier", "XP multiplier"},
		{"ServerName", "Server name"},
		{"EACEnabled", "EAC enabled"},
		{"MaxSpawnedZombies", "Max spawned zombies"},
		{"ServerWebsiteURL", "Server website URL"},
		{"DayNightLength", "Day night length"},
	} {
		if got := Label(tt.in); got != tt.want {
			t.Errorf("Label(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// EnableMapRendering is the reason this list exists: the console accepts the
// change, reports success, and the renderer stays off.
func TestASettingTheServerOnlyReadsAtStartupIsNotOffered(t *testing.T) {
	for _, name := range []string{
		"EnableMapRendering",
		"WebDashboardEnabled",
		"WebDashboardPort",
		"TelnetPort",
		"ServerPort",
		"GameWorld",
		"WorldGenSeed",
		"EACEnabled",
	} {
		if !StartupOnly(name) {
			t.Errorf("%s is read at startup but the panel would offer to change it", name)
		}
	}
}

// Marking a setting read-only when it is not is its own failure: it takes away
// a change the operator could have made.
func TestASettingTheRunningServerActsOnStaysEditable(t *testing.T) {
	for _, name := range []string{
		"MaxSpawnedZombies",
		"MaxSpawnedAnimals",
		"LandClaimSize",
		"LandClaimExpiryTime",
		"PlayerKillingMode",
		"BedrollDeadZoneSize",
		"BuildCreate",
		"AirDropFrequency",
		"MaxChunkAge",
	} {
		if StartupOnly(name) {
			t.Errorf("%s takes effect while the server runs but the panel refuses it", name)
		}
	}
}

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

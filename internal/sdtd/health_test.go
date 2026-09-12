package sdtd

import "testing"

// Captured verbatim from a live server running 3.2.0 (b10).
const memSample = `Time: 1314.44m FPS: 19.99 Heap: 1442.2MB Max: 2031.6MB Chunks: 253 CGO: 27 Ply: 0 Zom: 0 Ent: 3 (37) Items: 0 CO: 3 RSS: 2595.4MB
Observers
 id=2
 id=41`

const versionSample = `Game version: V 3.2.0 (b10) Compatibility Version: V 3.2.0
Mod TFP_Harmony: 1.1.0.4
Mod Allocs_Commands: 30
Mod Allocs_Webinterface: 52`

func TestParseMem(t *testing.T) {
	got := parseMem(memSample)

	for _, tc := range []struct {
		field string
		got   float64
		want  float64
	}{
		{"uptime", got.UptimeMinutes, 1314.44},
		{"fps", got.FPS, 19.99},
		{"heap", got.HeapMB, 1442.2},
		{"max heap", got.MaxHeapMB, 2031.6},
		{"rss", got.RSSMB, 2595.4},
		{"chunks", float64(got.Chunks), 253},
		{"players", float64(got.Players), 0},
		{"zombies", float64(got.Zombies), 0},
		{"entities", float64(got.Entities), 3},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %v, want %v", tc.field, tc.got, tc.want)
		}
	}
}

// A counter the panel does not know about must not shift the ones it does.
// The fields are matched by name for exactly this reason.
func TestParseMemToleratesNewFields(t *testing.T) {
	got := parseMem("Time: 10.0m Wibble: 42 FPS: 30.5 Heap: 100.0MB RSS: 200.0MB")

	if got.FPS != 30.5 {
		t.Errorf("fps = %v, want 30.5", got.FPS)
	}
	if got.RSSMB != 200 {
		t.Errorf("rss = %v, want 200", got.RSSMB)
	}
}

func TestParseVersion(t *testing.T) {
	var health Health
	parseVersion(versionSample, &health)

	if health.GameVersion != "V 3.2.0 (b10)" {
		t.Errorf("game version = %q, want %q", health.GameVersion, "V 3.2.0 (b10)")
	}
	if len(health.Mods) != 3 {
		t.Fatalf("mods = %d, want 3", len(health.Mods))
	}
	if health.Mods[0].Name != "TFP_Harmony" || health.Mods[0].Version != "1.1.0.4" {
		t.Errorf("first mod = %+v", health.Mods[0])
	}
}

// An empty or unexpected reply must read as "nothing known", not as a server
// sitting at zero frames a second.
func TestParseMemEmpty(t *testing.T) {
	got := parseMem("")
	if got.FPS != 0 || got.RSSMB != 0 || got.Chunks != 0 || got.UptimeMinutes != 0 {
		t.Errorf("parseMem(\"\") = %+v, want everything at zero", got)
	}
}

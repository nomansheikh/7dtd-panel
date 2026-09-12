package sdtd

import "testing"

// The parser reads a format nobody promised to keep, so it is pinned to a
// capture from a live server rather than to something plausible-looking.
func TestParseWeather(t *testing.T) {
	const captured = `WeatherManager #5
pine_forest: default, nxtT249010, Temperature 77.30975, Precipitation 0, CloudThickness 21.1739, Wind 4.690832, Fog 0.4519298, rain 0, snow 0, storm WT 249193, dur 2472, state 0
desert: stormbuild, nxtT0, Temperature 106.5059, Precipitation 0, CloudThickness 99.21471, Wind 35, Fog 5, rain 0, snow 0, storm WT 244363, dur 3128, state 1
snow: storm, nxtT0, Temperature 13.38085, Precipitation 0, CloudThickness 98.25443, Wind 50, Fog 8, rain 0, snow 0, storm WT 244314, dur 3380, state 2
Clouds 0
Fog density 0, start 20, end 80
FogColor 0.5 0.5 0.5
Rain 0.5
Snowfall 0
Temperature 0
Wind 0
`
	got := parseWeather(captured)

	if len(got.Biomes) != 3 {
		t.Fatalf("parsed %d biomes, want 3: %+v", len(got.Biomes), got.Biomes)
	}

	pine := got.Biomes[0]
	if pine.Biome != "pine_forest" || pine.State != "default" {
		t.Errorf("first biome = %+v", pine)
	}
	if pine.Temperature != 77.30975 || pine.Clouds != 21.1739 || pine.Wind != 4.690832 {
		t.Errorf("pine readings = %+v", pine)
	}
	if pine.Fog != 0.4519298 || pine.Rain != 0 || pine.Snow != 0 {
		t.Errorf("pine precipitation = %+v", pine)
	}

	// The state word is the thing an operator actually scans for.
	if got.Biomes[1].State != "stormbuild" || got.Biomes[2].State != "storm" {
		t.Errorf("states = %q, %q", got.Biomes[1].State, got.Biomes[2].State)
	}

	// The override block is separate from the biome readings, and Rain here is
	// forced while every biome reports no rain falling.
	if got.Overrides.Rain != 0.5 {
		t.Errorf("override rain = %v, want 0.5", got.Overrides.Rain)
	}
	if got.Overrides.Clouds != 0 || got.Overrides.Wind != 0 {
		t.Errorf("overrides = %+v", got.Overrides)
	}
	// "Fog density 0, start 20, end 80" must not be read as "Fog 0, start..."
	if got.Overrides.Fog != 0 {
		t.Errorf("override fog = %v", got.Overrides.Fog)
	}
}

// A server that answers with nothing recognisable must not produce a
// confident-looking empty reading.
func TestParseWeatherIgnoresNoise(t *testing.T) {
	got := parseWeather("Unknown command\n")
	if len(got.Biomes) != 0 {
		t.Errorf("biomes = %+v, want none", got.Biomes)
	}
}

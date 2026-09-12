package sdtd

import (
	"context"
	"regexp"
	"strconv"
	"strings"
)

// BiomeWeather is what the weather is actually doing in one biome.
type BiomeWeather struct {
	Biome string `json:"biome"`
	// State is the server's own word: default, stormbuild or storm.
	State       string  `json:"state"`
	Temperature float64 `json:"temperature"`
	Clouds      float64 `json:"clouds"`
	Wind        float64 `json:"wind"`
	Fog         float64 `json:"fog"`
	Rain        float64 `json:"rain"`
	Snow        float64 `json:"snow"`
}

// WeatherOverrides are the values an admin has forced, which sit on top of
// whatever the simulation would otherwise do.
type WeatherOverrides struct {
	Clouds      float64 `json:"clouds"`
	Fog         float64 `json:"fog"`
	Rain        float64 `json:"rain"`
	Snow        float64 `json:"snow"`
	Temperature float64 `json:"temperature"`
	Wind        float64 `json:"wind"`
}

// Weather is the whole picture: what is forced, and what each biome is doing.
type Weather struct {
	Overrides WeatherOverrides `json:"overrides"`
	Biomes    []BiomeWeather   `json:"biomes"`
}

// The bare weather command prints a block per biome and then the override
// values. Captured verbatim from a live server:
//
//	WeatherManager #5
//	pine_forest: default, nxtT249010, Temperature 77.3, Precipitation 0, CloudThickness 21.2, Wind 4.7, Fog 0.45, rain 0, snow 0, storm WT 249193, dur 2472, state 0
//	desert: stormbuild, nxtT0, Temperature 106.5, ...
//	Clouds 0
//	Fog density 0, start 20, end 80
//	FogColor 0.5 0.5 0.5
//	Rain 0.5
//	Snowfall 0
//	Temperature 0
//	Wind 0
var (
	biomeWeatherRe = regexp.MustCompile(
		`^(\w+):\s*(\w+),.*?Temperature\s*(-?[\d.]+),.*?CloudThickness\s*(-?[\d.]+),\s*Wind\s*(-?[\d.]+),\s*Fog\s*(-?[\d.]+),\s*rain\s*(-?[\d.]+),\s*snow\s*(-?[\d.]+)`)
	overrideRe   = regexp.MustCompile(`^(Clouds|Rain|Snowfall|Temperature|Wind)\s+(-?[\d.]+)\s*$`)
	fogDensityRe = regexp.MustCompile(`^Fog density\s+(-?[\d.]+)`)
)

// CurrentWeather reads what the weather is doing.
//
// There is no REST endpoint for this; the bare weather command is the only
// source. Worth parsing anyway, because the panel could previously set weather
// but never show it, which meant changing it blind.
//
// Note for anything rendering the overrides: the server reports a cleared
// override and an override deliberately set to zero identically, both as 0.
// After "weather Defaults" every value reads 0, and so does forcing rain to 0.
// The two cannot be told apart from this output.
func (c *Client) CurrentWeather(ctx context.Context) (Weather, error) {
	result, err := c.Execute(ctx, "weather")
	if err != nil {
		return Weather{}, err
	}
	return parseWeather(result.Result), nil
}

// parseWeather is split out so the format can be pinned to a captured sample
// in a test without needing a server.
func parseWeather(out string) Weather {
	var weather Weather
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)

		if m := biomeWeatherRe.FindStringSubmatch(line); m != nil {
			weather.Biomes = append(weather.Biomes, BiomeWeather{
				Biome:       m[1],
				State:       m[2],
				Temperature: toFloat(m[3]),
				Clouds:      toFloat(m[4]),
				Wind:        toFloat(m[5]),
				Fog:         toFloat(m[6]),
				Rain:        toFloat(m[7]),
				Snow:        toFloat(m[8]),
			})
			continue
		}

		if m := fogDensityRe.FindStringSubmatch(line); m != nil {
			weather.Overrides.Fog = toFloat(m[1])
			continue
		}

		if m := overrideRe.FindStringSubmatch(line); m != nil {
			value := toFloat(m[2])
			switch m[1] {
			case "Clouds":
				weather.Overrides.Clouds = value
			case "Rain":
				weather.Overrides.Rain = value
			case "Snowfall":
				weather.Overrides.Snow = value
			case "Temperature":
				weather.Overrides.Temperature = value
			case "Wind":
				weather.Overrides.Wind = value
			}
		}
	}
	return weather
}

func toFloat(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

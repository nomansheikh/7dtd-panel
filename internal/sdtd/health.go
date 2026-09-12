package sdtd

import (
	"context"
	"regexp"
	"strconv"
	"strings"
)

// Health is what the server says about itself: how hard it is working and what
// it is running.
//
// None of this is in the REST API. The mem and version commands are the only
// source, so it comes back through the console like the weather does.
type Health struct {
	// FPS is the server's own simulation rate. The number an operator actually
	// wants when players report the world feeling slow.
	FPS float64 `json:"fps"`
	// HeapMB and MaxHeapMB are the managed heap; RSSMB is the whole process as
	// the operating system sees it, which is the figure that matters when a
	// container hits its memory limit.
	HeapMB    float64 `json:"heapMb"`
	MaxHeapMB float64 `json:"maxHeapMb"`
	RSSMB     float64 `json:"rssMb"`
	// UptimeMinutes is how long the world has been running.
	UptimeMinutes float64 `json:"uptimeMinutes"`
	Chunks        int     `json:"chunks"`
	Players       int     `json:"players"`
	Zombies       int     `json:"zombies"`
	Entities      int     `json:"entities"`
	Items         int     `json:"items"`

	GameVersion string `json:"gameVersion"`
	Mods        []Mod  `json:"mods"`
}

// Mod is one loaded modlet, as the version command reports it.
type Mod struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// The mem command answers with a single dense line. Captured verbatim from a
// live server:
//
//	Time: 1314.44m FPS: 19.99 Heap: 1442.2MB Max: 2031.6MB Chunks: 253 CGO: 27
//	Ply: 0 Zom: 0 Ent: 3 (37) Items: 0 CO: 3 RSS: 2595.4MB
//
// Matched field by field rather than by position, because the set of fields
// has changed between game versions before and a positional parse would turn
// one new counter into a wrong number rather than a missing one.
var memFields = regexp.MustCompile(`(\w+):\s*([\d.]+)`)

// Game version: V 3.2.0 (b10) Compatibility Version: V 3.2.0
var (
	gameVersionRe = regexp.MustCompile(`Game version:\s*(.+?)\s+Compatibility`)
	modRe         = regexp.MustCompile(`^Mod\s+(\S+):\s*(.+)$`)
)

// ServerHealth reads the load and version figures.
//
// Two commands rather than one because the game has no single call for it.
// A failure of either is returned, since a half-filled reading would be
// reported as zeroes and read as a server doing nothing.
func (c *Client) ServerHealth(ctx context.Context) (Health, error) {
	mem, err := c.Execute(ctx, "mem")
	if err != nil {
		return Health{}, err
	}
	version, err := c.Execute(ctx, "version")
	if err != nil {
		return Health{}, err
	}
	health := parseMem(mem.Result)
	parseVersion(version.Result, &health)
	return health, nil
}

// parseMem is split out so the format can be pinned to a captured sample in a
// test without needing a server.
func parseMem(out string) Health {
	var health Health
	// Only the first line carries the counters; the rest lists chunk observers.
	line, _, _ := strings.Cut(strings.TrimSpace(out), "\n")
	for _, match := range memFields.FindAllStringSubmatch(line, -1) {
		value, err := strconv.ParseFloat(match[2], 64)
		if err != nil {
			continue
		}
		switch match[1] {
		case "Time":
			health.UptimeMinutes = value
		case "FPS":
			health.FPS = value
		case "Heap":
			health.HeapMB = value
		case "Max":
			health.MaxHeapMB = value
		case "RSS":
			health.RSSMB = value
		case "Chunks":
			health.Chunks = int(value)
		case "Ply":
			health.Players = int(value)
		case "Zom":
			health.Zombies = int(value)
		case "Ent":
			health.Entities = int(value)
		case "Items":
			health.Items = int(value)
		}
	}
	return health
}

func parseVersion(out string, health *Health) {
	if m := gameVersionRe.FindStringSubmatch(out); m != nil {
		health.GameVersion = strings.TrimSpace(m[1])
	}
	for _, line := range strings.Split(out, "\n") {
		if m := modRe.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			health.Mods = append(health.Mods, Mod{Name: m[1], Version: strings.TrimSpace(m[2])})
		}
	}
}

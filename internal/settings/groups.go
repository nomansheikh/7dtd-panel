// Package settings turns the game server's flat list of preferences into
// something a person can actually operate.
//
// The server exposes 287 preferences as one undifferentiated list of
// name/type/value/default. That is fine for a machine and useless for a human:
// the point of the panel over the in-game console is that nobody should have to
// know a setting is called AirDropFrequency, or that its value is an integer.
package settings

import (
	"strings"
	"unicode"
)

// Group is a heading in the settings UI.
type Group string

const (
	GroupServer     Group = "Server"
	GroupWorld      Group = "World"
	GroupBloodMoon  Group = "Blood moon"
	GroupZombies    Group = "Zombies"
	GroupLoot       Group = "Loot and air drops"
	GroupPlayers    Group = "Players"
	GroupLandClaim  Group = "Land claims"
	GroupDifficulty Group = "Difficulty and progression"
	GroupNetwork    Group = "Network and admin"
	GroupOther      Group = "Other"
)

// Order is the order groups appear. Explicit rather than alphabetical, so the
// things an operator changes most often come first and the grab-bags come
// last: "Server" is where everything without a better home ends up, including
// read-outs like GameVersion that are settings only in the sense that the game
// keeps them in the same list.
var Order = []Group{
	GroupPlayers, GroupWorld, GroupBloodMoon, GroupZombies, GroupLoot,
	GroupLandClaim, GroupDifficulty, GroupNetwork, GroupServer, GroupOther,
}

// prefixGroups maps a name prefix to its group. Longest match wins, so
// BloodMoonEnemyCount lands under Blood moon rather than Zombies.
var prefixGroups = []struct {
	prefix string
	group  Group
}{
	{"BloodMoon", GroupBloodMoon},
	{"ZombieBM", GroupBloodMoon},
	{"Zombie", GroupZombies},
	{"EnemySpawn", GroupZombies},
	{"EnemyDifficulty", GroupDifficulty},
	{"Enemy", GroupZombies},
	{"AirDrop", GroupLoot},
	{"Loot", GroupLoot},
	{"Land", GroupLandClaim},
	{"Claim", GroupLandClaim},
	{"Player", GroupPlayers},
	{"Party", GroupPlayers},
	{"Friend", GroupPlayers},
	{"Block", GroupWorld},
	{"World", GroupWorld},
	{"Day", GroupWorld},
	{"Bedroll", GroupWorld},
	{"Dynamic", GroupWorld},
	{"Web", GroupNetwork},
	{"Telnet", GroupNetwork},
	{"Server", GroupServer},
	{"Game", GroupServer},
	{"Admin", GroupNetwork},
	{"Control", GroupNetwork},
	{"EAC", GroupNetwork},
	{"Terminal", GroupNetwork},
	{"XP", GroupDifficulty},
	{"Difficulty", GroupDifficulty},
	{"Craft", GroupDifficulty},
	{"Build", GroupWorld},
	{"Max", GroupServer},
	{"Create", GroupServer},
	{"Save", GroupServer},
	{"Region", GroupServer},
	{"Language", GroupServer},
	{"Persistent", GroupPlayers},
	{"Drop", GroupPlayers},
	{"Twitch", GroupNetwork},
	{"Quest", GroupDifficulty},
	{"Biome", GroupWorld},
	{"Storm", GroupWorld},
	{"AI", GroupZombies},
	{"Spawn", GroupPlayers},
	{"Map", GroupWorld},
	{"Discord", GroupOther},
	{"Options", GroupOther},
	{"Debug", GroupOther},
}

// nameGroups place the handful of preferences whose names give no usable
// prefix. Checked before prefixes.
var nameGroups = map[string]Group{
	"DeathPenalty":            GroupPlayers,
	"JarRefund":               GroupDifficulty,
	"BiomeProgression":        GroupDifficulty,
	"CreativeMenuEnabled":     GroupNetwork,
	"EnableMapRendering":      GroupNetwork,
	"IgnoreEOSSanctions":      GroupNetwork,
	"JoiningOptions":          GroupNetwork,
	"HideCommandExecutionLog": GroupNetwork,
	"ResetUnprotectedChunks":  GroupWorld,
	"RebuildMap":              GroupWorld,
	"SkipSpawnButton":         GroupPlayers,
	"UserWorldStorageType":    GroupServer,
	"SandboxCode":             GroupServer,
	"SandboxPreset":           GroupServer,
	"NoGraphicsMode":          GroupServer,
	"LastGameResetRevision":   GroupServer,
	"AllowSpawnNearBackpack":  GroupPlayers,
	"AllowSpawnNearFriend":    GroupPlayers,
	"AutopilotMode":           GroupOther,
	"CameraRestrictionMode":   GroupOther,
	"FragLimit":               GroupOther,
	"MatchLength":             GroupOther,
}

// clientPrefixes mark preferences that belong to the game client rather than
// to a dedicated server: graphics, audio, controller bindings, the player's
// Discord account. Client and server share one preferences enum, so all 180 of
// them come back from /api/gameprefs on a headless server where they mean
// nothing. They are kept out of the way rather than dropped, because a mod
// could reasonably reuse one and an operator should still be able to see it.
var clientPrefixes = []string{
	"Options",
	"Discord",
	"UNUSED_",
	"Eula",
	"Playtest",
	"Selection",
	"Debug",
	"ConnectToServer",
	"LastLoaded",
	"LastLoadingTip",
	"FavoriteServers",
}

// ClientOnly reports whether a preference is the game client's rather than the
// server's.
func ClientOnly(name string) bool {
	for _, p := range clientPrefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

// GroupFor decides which heading a preference belongs under.
func GroupFor(name string) Group {
	if g, ok := nameGroups[name]; ok {
		return g
	}
	best := GroupOther
	bestLen := 0
	for _, pg := range prefixGroups {
		if len(pg.prefix) > bestLen && strings.HasPrefix(name, pg.prefix) {
			best, bestLen = pg.group, len(pg.prefix)
		}
	}
	return best
}

// acronyms stay upper case when a name is turned into a label.
var acronyms = []string{"XP", "EAC", "IP", "URL", "UI", "POI", "AI", "FPS", "NPC"}

// Label turns a preference name into something readable: AirDropFrequency
// becomes "Air drop frequency".
//
// Derived rather than kept in a table of 287 entries, because a table would
// silently lose anything a mod adds.
func Label(name string) string {
	if name == "" {
		return ""
	}

	var words []string
	var current strings.Builder
	runes := []rune(name)

	for i, r := range runes {
		// Split before a capital that starts a new word, keeping runs of
		// capitals such as XP and EAC together.
		if i > 0 && unicode.IsUpper(r) {
			prev := runes[i-1]
			nextLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
			if unicode.IsLower(prev) || unicode.IsDigit(prev) || (unicode.IsUpper(prev) && nextLower) {
				words = append(words, current.String())
				current.Reset()
			}
		}
		current.WriteRune(r)
	}
	words = append(words, current.String())

	// Sentence case, so a long list does not read as shouting.
	out := make([]string, 0, len(words))
	for i, w := range words {
		if isAcronym(w) {
			out = append(out, w)
			continue
		}
		if i == 0 {
			out = append(out, strings.ToUpper(w[:1])+strings.ToLower(w[1:]))
			continue
		}
		out = append(out, strings.ToLower(w))
	}
	joined := strings.Join(out, " ")
	if joined == "" {
		return name
	}
	return strings.ToUpper(joined[:1]) + joined[1:]
}

func isAcronym(w string) bool {
	for _, a := range acronyms {
		if strings.EqualFold(w, a) {
			return true
		}
	}
	return false
}

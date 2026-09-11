// Package console holds the panel's policy for executing game console commands.
//
// The game server will run anything the token's permission level allows, so
// every guard against accident or injection lives here rather than there.
package console

import (
	"fmt"
	"strings"
)

// Tier describes how much ceremony a command warrants before it runs.
type Tier string

const (
	// TierNormal is a read or an otherwise harmless command.
	TierNormal Tier = "normal"
	// TierMutating changes game state and gets a confirmation dialog.
	TierMutating Tier = "mutating"
	// TierDestructive can end the session or discard world data, and gets a
	// type-to-confirm dialog.
	TierDestructive Tier = "destructive"
)

// destructive commands can disconnect everyone or permanently discard world
// data. Naming them explicitly is deliberate: the alternative is guessing from
// the description text, which would silently miss anything a mod adds.
var destructive = map[string]Tier{
	"shutdown":             TierDestructive,
	"killall":              TierDestructive,
	"kickall":              TierDestructive,
	"regionreset":          TierDestructive,
	"worldchunkreset":      TierDestructive,
	"chunkreset":           TierDestructive,
	"damagereset":          TierDestructive,
	"resetallstats":        TierDestructive,
	"removelandprotection": TierDestructive,
}

// mutating commands change the world or a player but are recoverable.
var mutating = map[string]Tier{
	"ban": TierMutating, "kick": TierMutating, "kill": TierMutating,
	"give": TierMutating, "giveself": TierMutating, "givexp": TierMutating,
	"giveselfxp": TierMutating, "givequest": TierMutating,
	"teleport": TierMutating, "teleportplayer": TierMutating,
	"tppoi": TierMutating, "teleportpoirelative": TierMutating,
	"buff": TierMutating, "buffplayer": TierMutating,
	"debuff": TierMutating, "debuffplayer": TierMutating,
	"settime": TierMutating, "weather": TierMutating, "setgamepref": TierMutating,
	"setgamestat": TierMutating, "spawnentity": TierMutating,
	"spawnentityat": TierMutating, "spawnairdrop": TierMutating,
	"spawnsupplycrate": TierMutating, "spawnscouts": TierMutating,
	"spawnwandering": TierMutating, "saveworld": TierMutating,
	"whitelist": TierMutating, "admin": TierMutating, "say": TierMutating,
	"sayplayer": TierMutating, "starve": TierMutating, "thirsty": TierMutating,
	"exhausted": TierMutating, "creativemenu": TierMutating,
	"removequest": TierMutating, "unlock": TierMutating,
	"repairchunkdensity": TierMutating, "settempunit": TierMutating,
	"enablerendering": TierMutating, "rendermap": TierMutating,
	"createwebuser": TierMutating, "webtokens": TierMutating,
	"webpermission": TierMutating, "commandpermission": TierMutating,
}

// ClassifyTier reports the tier for a full command line.
//
// Only the first word is considered, because that is what the server dispatches
// on. Matching is case-insensitive, as the server's own command names are
// inconsistently cased.
func ClassifyTier(commandLine string) Tier {
	name := strings.ToLower(FirstWord(commandLine))
	if name == "" {
		return TierNormal
	}
	if tier, ok := destructive[name]; ok {
		return tier
	}
	if tier, ok := mutating[name]; ok {
		return tier
	}
	return TierNormal
}

// FirstWord returns the command name from a command line.
func FirstWord(commandLine string) string {
	trimmed := strings.TrimSpace(commandLine)
	if trimmed == "" {
		return ""
	}
	if i := strings.IndexAny(trimmed, " \t"); i >= 0 {
		return trimmed[:i]
	}
	return trimmed
}

// maxCommandLength bounds what will be sent to the game server. The longest
// legitimate command is far shorter than this.
const maxCommandLength = 1024

// Validate checks a free-form command line before it is sent.
//
// The console is intentionally unrestricted in which commands it will run, so
// this rejects only what would be malformed or would let one submission smuggle
// in a second command.
func Validate(commandLine string) error {
	trimmed := strings.TrimSpace(commandLine)
	if trimmed == "" {
		return fmt.Errorf("enter a command")
	}
	if len(trimmed) > maxCommandLength {
		return fmt.Errorf("command is %d characters; the limit is %d",
			len(trimmed), maxCommandLength)
	}
	// A newline would let a single submission run a second command that never
	// appeared in the confirmation dialog or the history.
	if strings.ContainsAny(trimmed, "\r\n") {
		return fmt.Errorf("command must be a single line")
	}
	if strings.ContainsRune(trimmed, '\x00') {
		return fmt.Errorf("command contains a null byte")
	}
	return nil
}

// IsDestructiveAllowed reports whether a destructive command may run under the
// current configuration.
func IsDestructiveAllowed(commandLine string, allowDestructive bool) bool {
	if allowDestructive {
		return true
	}
	return ClassifyTier(commandLine) != TierDestructive
}

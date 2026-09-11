package sdtd

import (
	"context"
	"regexp"
	"strings"
)

// Buff is one buff or debuff the server will apply to a player.
type Buff struct {
	Name string `json:"name"`
	// LocalizedName is the wording the game shows a player, absent for the
	// internal buffs that never surface in the UI.
	LocalizedName string `json:"localizedName,omitempty"`
}

// buffLine matches one entry of the list buffplayer prints when it is called
// without arguments: " - buffbrokenleg (Broken Leg)", or no parenthesis at all.
var buffLine = regexp.MustCompile(`^\s*-\s+(\S+)(?:\s+\((.*)\))?\s*$`)

// Buffs fetches the catalogue of buffs.
//
// There is no REST endpoint and no listbuffs command. What there is: calling
// buffplayer with no arguments is an error, and the error lists all 483 buffs
// with their display names. That is the only machine-readable source the server
// offers, and it is worth using, because the alternative is asking an operator
// to remember that "Broken Leg" is spelled buffbrokenleg.
func (c *Client) Buffs(ctx context.Context) ([]Buff, error) {
	result, err := c.Execute(ctx, "buffplayer")
	if err != nil {
		return nil, err
	}

	var buffs []Buff
	for _, line := range strings.Split(result.Result, "\n") {
		m := buffLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		buffs = append(buffs, Buff{Name: m[1], LocalizedName: strings.TrimSpace(m[2])})
	}
	return buffs, nil
}

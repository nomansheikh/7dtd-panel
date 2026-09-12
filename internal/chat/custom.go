package chat

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/nomansheikh/7dtd-panel/internal/console"
	"github.com/nomansheikh/7dtd-panel/internal/store"
)

/*
Commands an admin wrote.

The panel ships five of these and an admin will want a sixth within a day:
!discord, !rules, !starterkit, !home. So a custom command is a name, an
optional reply and an optional list of console lines, and there is no allowlist
of what those lines may be — the same person already has a console page in this
panel that runs anything they type, and refusing them the same command behind a
chat trigger would be a rule that exists nowhere else in the product.

What the panel owes them instead is visibility and a switch. The tier of every
line is shown before the command is turned on, PANEL_ALLOW_DESTRUCTIVE is
honoured here exactly as it is on the console page, and a new custom command
starts as admins-only so that it is never live to a whole server by accident.

The one thing held firmly is that a command runs what its author wrote. What a
player types can be substituted into a line, but never in a way that changes
the line's shape: otherwise the thing that ran is not the thing the admin
composed, and no amount of it being their own server makes that not a bug.
*/

// Placeholders an admin can put in a reply or a command line.
const (
	phEntityID   = "{entityid}"
	phPlayer     = "{player}"
	phPlatformID = "{platformid}"
	phArgs       = "{args}"
)

// Placeholders lists what may be used, for the panel's own help text.
var Placeholders = []struct {
	Token string `json:"token"`
	Means string `json:"means"`
}{
	{phEntityID, "The entity id of whoever typed it. This is what give, buff and teleport take."},
	{phPlayer, "Their name."},
	{phPlatformID, "Their platform id, such as Steam_7656…. This is what ban and admin take."},
	{phArgs, "Whatever they typed after the command name."},
	{"{arg1}", "Just the first word of it. {arg2} is the second, and so on."},
}

// argRe matches {arg1}, {arg2} and so on.
var argRe = regexp.MustCompile(`\{arg(\d+)\}`)

/*
safeInCommand is what a substituted value may contain when it lands in a
console command.

The same character set console.checkIdentifier allows, which covers every item,
buff and entity class name the game has, plus numbers and coordinates. A value
outside it is refused rather than escaped, for the reason the builders give: the
console documents no escape, so guessing at one turns an argument into two.
*/
var safeInCommand = regexp.MustCompile(`^[A-Za-z0-9_.\-]*$`)

// ErrUnsafeArgument is returned when what a player typed cannot go into a
// command line without changing its shape.
type ErrUnsafeArgument struct{ Value string }

func (e *ErrUnsafeArgument) Error() string {
	return "argument " + strconv.Quote(e.Value) + " is not a plain word"
}

/*
expandCommand fills the placeholders in one console line.

Every value that reaches a command is either an integer, or checked against
safeInCommand. A player called `bob"; shutdown` gets their name refused here
rather than quoted into somebody's command line.
*/
func expandCommand(line string, req Request) (string, error) {
	replace := func(token, value string) error {
		if !strings.Contains(line, token) {
			return nil
		}
		// Checked word by word. A space between two plain words is another
		// argument to the same command, which is exactly what an admin asked
		// for by writing {args}; it cannot turn a give into a shutdown. A
		// quote or a semicolon could, and does not get through.
		for _, word := range strings.Fields(value) {
			if !safeInCommand.MatchString(word) {
				return &ErrUnsafeArgument{value}
			}
		}
		line = strings.ReplaceAll(line, token, strings.Join(strings.Fields(value), " "))
		return nil
	}

	line = strings.ReplaceAll(line, phEntityID, strconv.Itoa(req.EntityID))
	if err := replace(phPlatformID, req.PlatformID); err != nil {
		return "", err
	}
	if err := replace(phPlayer, req.Player); err != nil {
		return "", err
	}
	if err := replace(phArgs, strings.Join(req.Args, " ")); err != nil {
		return "", err
	}

	var bad error
	line = argRe.ReplaceAllStringFunc(line, func(match string) string {
		n, err := strconv.Atoi(argRe.FindStringSubmatch(match)[1])
		if err != nil || n < 1 || n > len(req.Args) {
			// An absent argument becomes nothing, which leaves the command a
			// word short and the server to say so. Better than the panel
			// inventing a value.
			return ""
		}
		value := req.Args[n-1]
		if !safeInCommand.MatchString(value) {
			bad = &ErrUnsafeArgument{value}
			return ""
		}
		return value
	})
	if bad != nil {
		return "", bad
	}
	return line, nil
}

// expandReply fills the placeholders in the text a player is told.
//
// Nothing here becomes a command, so a name with a quote in it is only a name
// with a quote in it; sanitise strips what the console cannot carry.
func expandReply(text string, req Request) string {
	replacer := strings.NewReplacer(
		phEntityID, strconv.Itoa(req.EntityID),
		phPlayer, nameOrUnknown(req.Player),
		phPlatformID, req.PlatformID,
		phArgs, strings.Join(req.Args, " "),
	)
	text = replacer.Replace(text)
	return argRe.ReplaceAllStringFunc(text, func(match string) string {
		n, err := strconv.Atoi(argRe.FindStringSubmatch(match)[1])
		if err != nil || n < 1 || n > len(req.Args) {
			return ""
		}
		return req.Args[n-1]
	})
}

/*
runCustom carries out a command an admin wrote.

The lines are expanded before any of them is sent, so a command with a bad
placeholder in its third line does none of the first two rather than half the
job. Then they run in order, stopping at the first refusal — a sequence that
spawns a vehicle and teleports somebody into it should not do the second half
after the first has failed.
*/
func (b *Bot) runCustom(ctx context.Context, cfg store.ChatCommand, req Request) (string, error) {
	lines := make([]string, 0, len(cfg.Commands))
	for _, line := range cfg.Commands {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		expanded, err := expandCommand(line, req)
		if err != nil {
			return "", err
		}
		if err := console.Validate(expanded); err != nil {
			return "", fmt.Errorf("%s: %w", cfg.Name, err)
		}
		if !console.IsDestructiveAllowed(expanded, b.opts.AllowDestructive) {
			return "", fmt.Errorf("%s: %s is switched off in this panel",
				cfg.Name, console.FirstWord(expanded))
		}
		lines = append(lines, expanded)
	}

	for _, line := range lines {
		if _, err := b.opts.Client.Execute(ctx, line); err != nil {
			return "", fmt.Errorf("%s: %w", cfg.Name, err)
		}
	}

	if strings.TrimSpace(cfg.Reply) != "" {
		return expandReply(cfg.Reply, req), nil
	}
	if len(lines) == 0 {
		// Nothing to run and nothing to say. Saving one of these is possible
		// and pointless, so at least do not leave the player wondering.
		return "That command does nothing yet.", nil
	}
	// Acted but has nothing written to say. Silence would read as a command
	// that did not work.
	return "Done.", nil
}

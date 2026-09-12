package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/nomansheikh/7dtd-panel/internal/console"
	"github.com/nomansheikh/7dtd-panel/internal/store"
)

/*
The commands, fixed at compile time.

Deliberately a short list, and deliberately not a way to reach the console. The
panel can already run anything an operator types; what it could not do before
is answer a player who is holding a controller in a dark room and cannot alt-tab
to a browser. So each of these answers a question that is annoying to look up
mid-game, and only one of them changes anything.

Anything that could end a session or discard world data is absent, and cannot
be added by configuration: there is no row an operator can write that makes
!kickall exist.
*/

// Spec describes one command. The UI reads these to build its list, so the
// summaries are written for an operator deciding whether to switch it on.
type Spec struct {
	Name string `json:"name"`
	// Usage is how a player types it, for the help text.
	Usage   string `json:"usage"`
	Summary string `json:"summary"`
	// Acts is true when the command changes game state rather than reporting
	// it. Shown in the panel, because "everyone may run this" means something
	// different for the one command that hands out loot.
	Acts bool `json:"acts"`
	// DefaultCooldownSeconds is what the panel proposes when the command is
	// first switched on.
	DefaultCooldownSeconds int `json:"defaultCooldownSeconds"`

	run func(ctx context.Context, b *Bot, req Request) (string, error)
}

var specs = []Spec{
	{
		Name:    "help",
		Usage:   "!help",
		Summary: "Lists the commands that player is allowed to use.",
	},
	{
		Name:                   "day",
		Usage:                  "!day",
		Summary:                "Answers what day and time it is, and how long until dark.",
		DefaultCooldownSeconds: 10,
	},
	{
		Name:                   "bloodmoon",
		Usage:                  "!bloodmoon",
		Summary:                "Answers how many days until the next blood moon.",
		DefaultCooldownSeconds: 10,
	},
	{
		Name:                   "players",
		Usage:                  "!players",
		Summary:                "Lists who is online right now.",
		DefaultCooldownSeconds: 10,
	},
	{
		Name:                   "kit",
		Usage:                  "!kit <name>",
		Summary:                "Hands over a kit the panel has saved. Drops the items at the player's feet.",
		Acts:                   true,
		DefaultCooldownSeconds: 3600,
	},
}

/*
The handlers are attached here rather than in the literal above.

!help reads the list to work out what to tell somebody, and a list that named
its own handlers while one of those handlers read the list is an initialisation
cycle the compiler rejects. Attaching them afterwards is the smallest way out,
and keeps the table above readable as a table.
*/
func init() {
	handlers := map[string]func(context.Context, *Bot, Request) (string, error){
		"help":      runHelp,
		"day":       runDay,
		"bloodmoon": runBloodmoon,
		"players":   runPlayers,
		"kit":       runKit,
	}
	for i := range specs {
		run, ok := handlers[specs[i].Name]
		if !ok {
			panic("chat: no handler for " + specs[i].Name)
		}
		specs[i].run = run
	}
	if len(handlers) != len(specs) {
		panic("chat: a handler exists for a command that is not listed")
	}
}

// Specs returns every command the panel knows how to answer, in the order the
// UI should list them.
func Specs() []Spec {
	out := make([]Spec, len(specs))
	copy(out, specs)
	return out
}

func lookup(name string) (Spec, bool) {
	for _, s := range specs {
		if s.Name == name {
			return s, true
		}
	}
	return Spec{}, false
}

// runHelp lists what this particular player may run.
//
// Filtered by audience rather than printed whole: telling everybody about a
// command they will be refused is worse than not mentioning it.
func runHelp(ctx context.Context, b *Bot, req Request) (string, error) {
	configured, err := b.config(ctx)
	if err != nil {
		return "", err
	}

	var admin bool
	var checked bool
	var usable []string
	for _, spec := range specs {
		cfg, ok := configured[spec.Name]
		if !ok || !cfg.Enabled {
			continue
		}
		if cfg.Audience == store.AudienceAdmins {
			if !checked {
				admin, err = b.isAdmin(ctx, req.PlatformID)
				if err != nil {
					return "", err
				}
				checked = true
			}
			if !admin {
				continue
			}
		}
		usable = append(usable, Prefix+strings.TrimPrefix(spec.Usage, Prefix))
	}

	if len(usable) == 0 {
		return "Nothing is switched on for you right now.", nil
	}
	return "You can use: " + strings.Join(usable, ", "), nil
}

// dawnHour is when the game's day starts. Fixed in the game; the length of the
// day is not, and comes from the server's own DayLightLength setting.
const dawnHour = 4

func runDay(_ context.Context, b *Bot, _ Request) (string, error) {
	snap := b.opts.Poller.Snapshot()
	if snap.StatsAt.IsZero() {
		return "I do not know yet — the panel has not heard from the server.", nil
	}

	t := snap.Stats.GameTime
	daylight := snap.DaylightHours
	if daylight <= 0 || daylight >= 24 {
		daylight = 18
	}
	dusk := dawnHour + daylight

	now := float64(t.Hours) + float64(t.Minutes)/60
	lit := now >= dawnHour && now < float64(dusk)

	var until string
	if lit {
		until = fmt.Sprintf("Dark in %s.", gameHours(float64(dusk)-now, snap.DayMinutes))
	} else {
		next := float64(dawnHour)
		if now >= float64(dusk) {
			next += 24
		}
		until = fmt.Sprintf("Light in %s.", gameHours(next-now, snap.DayMinutes))
	}

	return fmt.Sprintf("Day %d, %02d:%02d. %s", t.Days, t.Hours, t.Minutes, until), nil
}

/*
gameHours renders a stretch of game time, with the real time it takes.

Both, because neither alone is the answer. "Four hours until dark" sounds like
an evening and is eleven real minutes on a default server; "eleven minutes"
does not tell a player whether they can finish the building they are standing
in. The conversion uses the server's own DayNightLength when it is known.
*/
func gameHours(hours float64, dayMinutes int) string {
	whole := int(hours)
	minutes := int((hours - float64(whole)) * 60)

	var game string
	switch {
	case whole == 0:
		game = plural(maxInt(minutes, 1), "game minute")
	case minutes == 0:
		game = plural(whole, "game hour")
	default:
		game = fmt.Sprintf("%dh%02dm of game time", whole, minutes)
	}
	if dayMinutes <= 0 {
		return game
	}
	real := time.Duration(hours / 24 * float64(dayMinutes) * float64(time.Minute))
	return fmt.Sprintf("%s (%s real)", game, humanWait(real))
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func runBloodmoon(_ context.Context, b *Bot, _ Request) (string, error) {
	snap := b.opts.Poller.Snapshot()
	if snap.BloodmoonAt.IsZero() {
		return "I do not know yet — the panel has not heard from the server.", nil
	}
	if snap.Bloodmoon.Active {
		return "It is happening right now. Good luck.", nil
	}

	next := snap.Bloodmoon.Next.Days
	away := next - snap.Bloodmoon.GameTime.Days
	switch {
	case away <= 0:
		return fmt.Sprintf("Tonight. Day %d — be somewhere you like.", next), nil
	case away == 1:
		return fmt.Sprintf("Tomorrow night, day %d.", next), nil
	default:
		return fmt.Sprintf("Day %d, which is %s away.", next, plural(away, "day")), nil
	}
}

// maxNames is how many players are listed before the rest become a count. A
// full server's worth of names would be truncated mid-name otherwise.
const maxNames = 12

func runPlayers(ctx context.Context, b *Bot, _ Request) (string, error) {
	players, err := b.opts.Client.Players(ctx)
	if err != nil {
		return "", err
	}

	var names []string
	for _, p := range players {
		if p.Online && p.Name != "" {
			names = append(names, p.Name)
		}
	}
	if len(names) == 0 {
		return "Nobody is online, which cannot be right if you are reading this.", nil
	}
	sort.Strings(names)

	count := plural(len(names), "player") + " online: "
	if len(names) > maxNames {
		return count + strings.Join(names[:maxNames], ", ") +
			fmt.Sprintf(" and %d more.", len(names)-maxNames), nil
	}
	return count + strings.Join(names, ", ") + ".", nil
}

// kitName is what a kit may be called. Kept to the same shape as an identifier
// so that a kit name can never be mistaken for a second console argument.
var kitName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,31}$`)

// KitItem is one line of a saved kit.
type KitItem struct {
	Item  string `json:"item"`
	Count int    `json:"count"`
	// Quality below 1 means the item has none, or that the game should decide.
	Quality int `json:"quality"`
}

// maxKitCommands bounds how many gives one kit may run. A kit is a convenience,
// not a way to make the panel send a hundred commands on a stranger's word.
const maxKitCommands = 25

func runKit(ctx context.Context, b *Bot, req Request) (string, error) {
	if len(req.Args) != 1 {
		return "Say " + Prefix + "kit and the name of one, like " + Prefix + "kit starter.", nil
	}
	name := strings.ToLower(req.Args[0])
	if !kitName.MatchString(name) {
		return "That is not a kit name.", nil
	}

	kit, err := b.opts.Store.Kit(ctx, name)
	if errors.Is(err, store.ErrNotFound) {
		return b.kitNames(ctx)
	}
	if err != nil {
		return "", err
	}

	var items []KitItem
	if err := json.Unmarshal([]byte(kit.Items), &items); err != nil {
		return "", fmt.Errorf("kit %s is not readable: %w", name, err)
	}
	if len(items) == 0 {
		return "The " + name + " kit is empty.", nil
	}
	if len(items) > maxKitCommands {
		return "", fmt.Errorf("kit %s has %d items; the limit is %d", name, len(items), maxKitCommands)
	}

	// Built before any of them are sent, so a kit containing one bad name hands
	// over nothing rather than half of itself.
	commands := make([]string, 0, len(items))
	total := 0
	for _, item := range items {
		count := item.Count
		if count < 1 {
			count = 1
		}
		command, err := console.GiveItem(req.EntityID, item.Item, count, item.Quality)
		if err != nil {
			return "", fmt.Errorf("kit %s: %w", name, err)
		}
		commands = append(commands, command)
		total += count
	}

	for _, command := range commands {
		if _, err := b.opts.Client.Execute(ctx, command); err != nil {
			return "", fmt.Errorf("kit %s: %w", name, err)
		}
	}

	return fmt.Sprintf("The %s kit is at your feet — %s.", name, plural(total, "item")), nil
}

// kitNames answers a request for a kit that does not exist by naming the ones
// that do, which is what the player was going to ask next.
func (b *Bot) kitNames(ctx context.Context) (string, error) {
	kits, err := b.opts.Store.Kits(ctx)
	if err != nil {
		return "", err
	}
	if len(kits) == 0 {
		return "There are no kits yet.", nil
	}
	names := make([]string, 0, len(kits))
	for _, kit := range kits {
		names = append(names, kit.Name)
	}
	return "No such kit. There is: " + strings.Join(names, ", ") + ".", nil
}

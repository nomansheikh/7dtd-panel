/*
Package chat answers players from inside the game.

The game server streams chat out over its log and takes console commands in,
and nothing joins the two: there is no notion of a bot, a command prefix or a
reply. The panel already sits in the middle holding both ends, so it is the
only thing that can close the loop — a player types !kit starter, the panel
recognises it, decides whether they may, and runs the gives.

Three rules shape everything here.

Silence is the default. A command with no row in the database is off, an
unknown !word gets no answer at all, and a disabled command is indistinguishable
from one that was never built. A panel that starts talking to strangers the
moment it is upgraded is a bad surprise on somebody else's server.

The panel never answers itself. The server's own broadcasts come back through
the same log as chat, as entity id -1, so anything that reacts to chat and can
also speak has to skip them or it will loop forever. That check is in one place,
in parse, and is the reason events.Event carries the entity id at all.

Nothing here is destructive. The set of commands is fixed at compile time and
none of them can kick, ban, wipe or shut anything down. An operator granting
!kit to everyone is granting exactly that, and cannot be talked into more by
a cleverly worded message.
*/
package chat

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/nomansheikh/7dtd-panel/internal/console"
	"github.com/nomansheikh/7dtd-panel/internal/events"
	"github.com/nomansheikh/7dtd-panel/internal/sdtd"
	"github.com/nomansheikh/7dtd-panel/internal/state"
	"github.com/nomansheikh/7dtd-panel/internal/store"
)

// Prefix marks a chat line as aimed at the panel.
//
// One character, and the one every game server bot has used for twenty years,
// so a player who has been on any other server already knows it.
const Prefix = "!"

// The collaborators are interfaces so a test can drive a whole conversation
// without a game server, a poller or an HTTP round trip anywhere in it.

// Executor runs console commands and reads the player list.
type Executor interface {
	Execute(ctx context.Context, command string) (sdtd.CommandResult, error)
	Players(ctx context.Context) ([]sdtd.Player, error)
}

// Snapshotter supplies the cached view of the world, which is where the
// read-only answers come from: asking the game again for a clock the panel
// already polls would add a round trip and could fail, where the cache cannot.
type Snapshotter interface {
	Snapshot() state.Snapshot
}

// Feed is the event hub the bot listens to.
type Feed interface {
	Subscribe(backlog int) ([]events.Event, <-chan events.Event, func())
}

// Announcer puts a line in the panel's own feed. Optional: an operator watching
// the console should see that somebody was handed a kit at 3am, but the bot
// works without one.
type Announcer interface {
	PublishStatus(message string)
}

// Store is the configuration and the kits.
type Store interface {
	ChatCommands(ctx context.Context, serverID string) ([]store.ChatCommand, error)
	TakeCooldown(ctx context.Context, serverID, platformID, command string, cooldown time.Duration, now time.Time) (bool, time.Duration, error)
	ClearCooldown(ctx context.Context, serverID, platformID, command string) error
	Kit(ctx context.Context, name string) (store.Kit, error)
	Kits(ctx context.Context) ([]store.Kit, error)
}

// Options configures a Bot.
type Options struct {
	// Server is the configured server id. Every command is configured per
	// server, so this is what the rules are looked up under, not decoration.
	Server   string
	Feed     Feed
	Client   Executor
	Poller   Snapshotter
	Store    Store
	Announce Announcer
	Logger   *slog.Logger
	// Now defaults to time.Now and exists so cooldowns are testable.
	Now func() time.Time
}

// Bot watches one server's chat and answers it.
type Bot struct {
	opts Options
	now  func() time.Time
	log  *slog.Logger

	// slots bounds how many commands may be in flight. The read loop must never
	// block: the hub drops a subscriber that stops reading, so a bot waiting on
	// a slow give would eventually deafen itself.
	slots chan struct{}

	adminMu  sync.Mutex
	adminIDs map[string]bool
	adminAt  time.Time
}

// concurrent is how many commands may run at once. A kit is several gives and
// takes a moment; four at a time is more than a full server will ever need, and
// a fifth simultaneous request is far more likely to be a keyboard held down
// than four people who all meant it.
const concurrent = 4

// replyTimeout bounds one command end to end, including the reply.
const replyTimeout = 15 * time.Second

// New builds a Bot. It contacts nothing.
func New(opts Options) *Bot {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Logger == nil {
		opts.Logger = slog.New(slog.DiscardHandler)
	}
	return &Bot{
		opts:  opts,
		now:   opts.Now,
		log:   opts.Logger,
		slots: make(chan struct{}, concurrent),
	}
}

/*
Run listens until ctx is cancelled.

Subscribing with no backlog is deliberate. The hub keeps two thousand lines of
scrollback, and a bot that read them on startup would answer every command sent
in the last hour the moment the panel restarted — including the ones it already
answered before it went down.
*/
func (b *Bot) Run(ctx context.Context) {
	for ctx.Err() == nil {
		_, ch, cancel := b.opts.Feed.Subscribe(0)
		b.listen(ctx, ch)
		cancel()
	}
}

// listen consumes one subscription. It returns when the context is cancelled or
// the hub drops the subscription, which it does to a client that has stopped
// reading — recoverable, so Run simply subscribes again.
func (b *Bot) listen(ctx context.Context, ch <-chan events.Event) {
	for {
		select {
		case <-ctx.Done():
			return
		case e, ok := <-ch:
			if !ok {
				b.log.Warn("chat bot fell behind the event feed; resubscribing")
				return
			}
			b.dispatch(ctx, e)
		}
	}
}

// dispatch hands one event to a worker, or drops it if every worker is busy.
//
// Dropping is the right failure: the alternative is a queue that grows while
// the game server is slow, and then empties into it all at once.
func (b *Bot) dispatch(parent context.Context, e events.Event) {
	req, ok := parse(e)
	if !ok {
		return
	}
	select {
	case b.slots <- struct{}{}:
	default:
		b.log.Warn("chat command dropped; the bot is busy",
			"server", b.opts.Server, "command", req.Name, "player", req.Player)
		return
	}
	go func() {
		defer func() { <-b.slots }()
		ctx, cancel := context.WithTimeout(parent, replyTimeout)
		defer cancel()
		b.Handle(ctx, req)
	}()
}

// Request is one command somebody typed.
type Request struct {
	Player     string
	EntityID   int
	PlatformID string
	// Name is the command without its prefix, lowercased.
	Name string
	Args []string
}

// parse recognises a command in a chat event.
//
// Everything that is not a command aimed at the panel is rejected here, so no
// handler has to think about it.
func parse(e events.Event) (Request, bool) {
	if e.Kind != events.KindChat {
		return Request{}, false
	}
	// The server's own broadcasts arrive as entity id -1. Answering one would
	// be answering ourselves, and a reply that is itself a command would never
	// stop.
	if e.EntityID == nil || *e.EntityID < 0 || e.PlatformID == "" {
		return Request{}, false
	}
	text := strings.TrimSpace(e.Message)
	if !strings.HasPrefix(text, Prefix) {
		return Request{}, false
	}
	fields := strings.Fields(text[len(Prefix):])
	if len(fields) == 0 {
		return Request{}, false
	}
	return Request{
		Player:     e.Player,
		EntityID:   *e.EntityID,
		PlatformID: e.PlatformID,
		Name:       strings.ToLower(fields[0]),
		Args:       fields[1:],
	}, true
}

/*
Handle runs one command, if it is allowed.

Exported so a test can drive a request without a hub, and so the API can offer
an operator a way to try a command as themselves.

Every refusal that is not the player's fault is silent. A command that does not
exist, or that the operator has turned off, produces nothing: telling somebody
"that command is disabled" is telling them it exists, which invites them to ask
for it. A refusal the player can act on — wait, or ask an admin — is spoken.
*/
func (b *Bot) Handle(ctx context.Context, req Request) {
	spec, ok := lookup(req.Name)
	if !ok {
		return
	}

	configured, err := b.config(ctx)
	if err != nil {
		b.log.Warn("could not read chat command config", "server", b.opts.Server, "error", err)
		return
	}
	cfg, ok := configured[spec.Name]
	if !ok || !cfg.Enabled {
		return
	}

	if cfg.Audience == store.AudienceAdmins {
		admin, err := b.isAdmin(ctx, req.PlatformID)
		if err != nil {
			b.log.Warn("could not read the admin list", "server", b.opts.Server, "error", err)
			return
		}
		if !admin {
			b.reply(ctx, req, "Only admins can use "+Prefix+spec.Name+".")
			return
		}
	}

	cooldown := time.Duration(cfg.CooldownSeconds) * time.Second
	allowed, left, err := b.opts.Store.TakeCooldown(ctx,
		b.opts.Server, req.PlatformID, spec.Name, cooldown, b.now())
	if err != nil {
		b.log.Warn("could not take the cooldown", "server", b.opts.Server, "error", err)
		return
	}
	if !allowed {
		b.reply(ctx, req, fmt.Sprintf("Not yet — try %s%s again in %s.", Prefix, spec.Name, humanWait(left)))
		return
	}

	answer, err := spec.run(ctx, b, req)
	if err != nil {
		// The cooldown was taken before the command ran, so that two copies of
		// the same request cannot both pass the check. Since this one achieved
		// nothing, give it back rather than charging an hour for a failure.
		if clearErr := b.opts.Store.ClearCooldown(ctx, b.opts.Server, req.PlatformID, spec.Name); clearErr != nil {
			b.log.Warn("could not refund the cooldown", "server", b.opts.Server, "error", clearErr)
		}
		b.log.Warn("chat command failed",
			"server", b.opts.Server, "command", spec.Name, "player", req.Player, "error", err)
		b.reply(ctx, req, "That did not work. An admin can see why in the panel.")
		return
	}

	b.reply(ctx, req, answer)
	if spec.Acts {
		b.announce(fmt.Sprintf("chat: %s ran %s%s", nameOrUnknown(req.Player), Prefix, spec.Name))
	}
}

// config reads every configured command, keyed by name.
func (b *Bot) config(ctx context.Context) (map[string]store.ChatCommand, error) {
	rows, err := b.opts.Store.ChatCommands(ctx, b.opts.Server)
	if err != nil {
		return nil, err
	}
	out := make(map[string]store.ChatCommand, len(rows))
	for _, row := range rows {
		out[row.Name] = row
	}
	return out, nil
}

// adminTTL is how long the admin list is trusted between reads. Long enough
// that a player holding a key down does not turn into a console command per
// keystroke; short enough that promoting somebody takes effect while they are
// still watching.
const adminTTL = 30 * time.Second

// adminID matches a platform user id in the admin list's output. Verified
// against a live server, whose populated list reads:
//
//	1: Steam_76561198803325430 (stored name: )
//
// Matching the id shape rather than the line shape means a build that changes
// the surrounding text, or adds a column, still resolves admins correctly.
var adminID = regexp.MustCompile(`\b[A-Za-z][A-Za-z0-9]*_[A-Za-z0-9]{4,}\b`)

// isAdmin reports whether a platform id is on the game's own admin list.
//
// The game's list, not the panel's logins: the people typing in chat have game
// identities and have never seen the panel.
func (b *Bot) isAdmin(ctx context.Context, platformID string) (bool, error) {
	b.adminMu.Lock()
	fresh := b.adminIDs != nil && b.now().Sub(b.adminAt) < adminTTL
	ids := b.adminIDs
	b.adminMu.Unlock()

	if !fresh {
		result, err := b.opts.Client.Execute(ctx, console.ListAdmins())
		if err != nil {
			return false, err
		}
		ids = make(map[string]bool)
		for _, match := range adminID.FindAllString(result.Result, -1) {
			ids[strings.ToLower(match)] = true
		}
		b.adminMu.Lock()
		b.adminIDs, b.adminAt = ids, b.now()
		b.adminMu.Unlock()
	}
	return ids[strings.ToLower(platformID)], nil
}

// reply says something to one player.
//
// sayplayer and not say: an answer to one person does not belong in everybody
// else's chat. It also never comes back through the log, which is convenient
// but not what the loop guard relies on.
func (b *Bot) reply(ctx context.Context, req Request, message string) {
	command, err := console.SayPlayer(req.EntityID, sanitise(message))
	if err != nil {
		b.log.Error("chat reply was rejected before sending",
			"server", b.opts.Server, "error", err)
		return
	}
	if _, err := b.opts.Client.Execute(ctx, command); err != nil {
		b.log.Warn("could not reply in chat", "server", b.opts.Server, "error", err)
	}
}

func (b *Bot) announce(message string) {
	if b.opts.Announce != nil {
		b.opts.Announce.PublishStatus(message)
	}
}

// maxReply is the longest message that will be sent. The builder rejects
// anything over 200 characters, and a reply losing its last word to a player
// name is a worse outcome than one that says "and 3 more".
const maxReply = 190

/*
sanitise makes a message safe to put inside a quoted console command.

Replies contain names players chose for themselves, so this is not cosmetic.
Quotes are removed rather than escaped because the console documents no escape;
a name containing one would otherwise end the argument early and hand the rest
of the line to the command parser.
*/
func sanitise(message string) string {
	cleaned := strings.Map(func(r rune) rune {
		switch r {
		case '"', '\'', '\r', '\n', '\x00':
			return -1
		}
		return r
	}, message)
	cleaned = strings.TrimSpace(cleaned)
	if cleaned == "" {
		return "…"
	}
	return clamp(cleaned, maxReply)
}

// clamp shortens to at most n bytes, on a rune boundary.
//
// Bytes rather than runes because that is what the builder's limit counts, and
// a message of two hundred accented characters would otherwise be rejected
// after this had declared it short enough.
func clamp(s string, n int) string {
	if len(s) <= n {
		return s
	}
	const ellipsis = "…"
	budget := n - len(ellipsis)
	runes := []rune(s)
	for len(runes) > 0 && len(string(runes)) > budget {
		runes = runes[:len(runes)-1]
	}
	return strings.TrimRight(string(runes), " ,") + ellipsis
}

// humanWait renders a remaining cooldown the way somebody would say it.
//
// Always rounded up: "try again in 0 minutes" is a lie that gets a second
// refusal a moment later.
func humanWait(d time.Duration) string {
	if d < time.Minute {
		s := int((d + time.Second - 1) / time.Second)
		if s < 1 {
			s = 1
		}
		return plural(s, "second")
	}
	if d < time.Hour {
		return plural(int((d+time.Minute-1)/time.Minute), "minute")
	}
	hours := int(d / time.Hour)
	minutes := int((d%time.Hour + time.Minute - 1) / time.Minute)
	if minutes == 0 {
		return plural(hours, "hour")
	}
	return plural(hours, "hour") + " " + plural(minutes, "minute")
}

func plural(n int, unit string) string {
	if n == 1 {
		return "1 " + unit
	}
	return fmt.Sprintf("%d %ss", n, unit)
}

func nameOrUnknown(name string) string {
	if name == "" {
		return "somebody"
	}
	return name
}

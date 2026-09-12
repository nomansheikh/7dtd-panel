import { useState, type ReactNode } from "react";
import { Link, useParams } from "react-router-dom";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { ArrowLeft, ShieldCheck } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { ItemPicker, type Pick } from "@/components/item-picker";
import { PlayerActionDialog, type PendingAction } from "@/components/player-actions";
import { useEvents } from "@/hooks/use-events";
import { usePlayerAction, usePlayers } from "@/hooks/use-players";
import { useServerId } from "@/hooks/use-servers";
import { api, ApiError, type ActionResult, type Player } from "@/lib/api";
import { formatAge, formatUptime } from "@/lib/format";
import { cn } from "@/lib/utils";

/**
 * One player, laid out as a deck rather than a stack.
 *
 * Two columns that fill the viewport, the same shape the overview uses: what
 * is true about somebody on the left, what can be done to them in a rail on
 * the right. The first version ran everything down one narrow column with the
 * whole right of the screen empty and the actions as a flat row of ghost
 * buttons — which is the layout this panel has had to be talked out of on
 * every page so far.
 *
 * Splitting it this way is not just about filling space. Reading a record and
 * acting on it are different jobs: the left side is evidence and the right is
 * consequence, and keeping the dangerous half in one column means it is never
 * somewhere you scroll past by accident.
 *
 * Keyed by platform id rather than entity id. An entity id only exists while
 * somebody is connected, and half of what is here — bans, admin levels, the
 * whitelist — is exactly what you reach for when they are not.
 */
/*
  Admin levels, named.

  The game has a number from 0 to 1000 and no names for any of it: 0 is the
  most access, 1000 is what everybody has without an entry, and what each level
  can actually do is defined command by command in the server's own
  serveradmin.xml. Three bare digits in a row told an operator nothing about
  which one to press.

  So these are the panel's words for the conventional three, kept beside the
  number rather than instead of it — the number is what the server stores and
  what an operator will see everywhere else.
*/
const ADMIN_LEVELS = [
  { level: 0, label: "Owner", hint: "Full access: every command the server has." },
  { level: 1, label: "Admin", hint: "Nearly everything. A rung below the owner." },
  { level: 2, label: "Moderator", hint: "The day-to-day commands: kick, ban, teleport." },
];

export function PlayerPage() {
  const { platformId = "" } = useParams();
  const serverId = useServerId();
  const queryClient = useQueryClient();
  const { data, isLoading } = usePlayers();
  const action = usePlayerAction();
  const [pending, setPending] = useState<PendingAction | null>(null);
  const [giving, setGiving] = useState(false);

  const player = data?.players.find((p) => p.platformId === platformId);

  const { data: access } = useQuery({
    queryKey: ["access", serverId],
    queryFn: () => api.access(serverId),
    enabled: serverId !== "",
  });

  const act = useMutation({
    mutationFn: ({ run }: { done: string; run: () => Promise<ActionResult> }) => run(),
    onSuccess: (result, { done }) => {
      toast.success(done, { description: result.command });
      void queryClient.invalidateQueries({ queryKey: ["access", serverId] });
      void queryClient.invalidateQueries({ queryKey: ["players", serverId] });
    },
    onError: (error) =>
      toast.error("That did not work", {
        description: error instanceof ApiError ? error.message : "The action failed.",
      }),
  });

  if (isLoading && !data) {
    return <Skeleton className="m-4 h-96 rounded-none" />;
  }

  if (!player) {
    return (
      <div className="flex h-full items-center justify-center p-8">
        <div className="text-center">
          <p className="text-sm text-bone-dim">The panel does not know that player.</p>
          <Button asChild variant="ghost" size="sm" className="mt-3">
            <Link to="/players">Back to everyone</Link>
          </Button>
        </div>
      </div>
    );
  }

  /*
    Whether the server's own listing mentions this player.

    A substring match on the id rather than a parse of the row. Checked against
    a live server, a populated entry reads "1: Steam_7656… (stored name: )" —
    but an id either appears in that text or it does not, and that holds however
    the rows are arranged or whatever a future build changes them to.
  */
  const mentions = (text: string | undefined) =>
    Boolean(
      text &&
      ((player.platformId && text.includes(player.platformId)) ||
        (player.crossplatformId && text.includes(player.crossplatformId))),
    );
  const isAdmin = mentions(access?.admins);
  const isWhitelisted = mentions(access?.whitelist);

  const offline = !player.online;
  const run = (done: string, fn: () => Promise<ActionResult>) => act.mutate({ done, run: fn });

  const online = (data?.players ?? []).filter((p) => p.online);

  /*
    Handing over a basket.

    The game's give command takes one item, so several of them is several
    commands run in order, and giving the same basket to everybody online is
    that again per person. Reported as one thing, because from where the
    operator is standing it was one thing.
  */
  const giveBasket = (picks: Pick[], everyone: boolean) => {
    const targets = everyone ? online : [player];
    const lines = picks.reduce((sum, p) => sum + p.count, 0);
    setGiving(false);
    run(
      everyone
        ? `${lines} items given to ${targets.length} players`
        : `${lines} items given to ${player.name}`,
      async () => {
        let last: ActionResult | undefined;
        const ran: string[] = [];
        for (const target of targets) {
          for (const pick of picks) {
            last = await api.giveItem(
              serverId,
              target.entityId,
              pick.item.name,
              pick.count,
              pick.quality,
            );
            ran.push(last.command);
          }
        }
        return { ...last!, command: ran.join(", ") };
      },
    );
  };

  return (
    <div className="grid h-full min-h-0 grid-rows-[auto_1fr] overflow-y-auto lg:grid-cols-[1fr_23rem] lg:grid-rows-1 lg:overflow-hidden">
      <div className="flex min-h-0 min-w-0 flex-col lg:overflow-y-auto">
        <header className="region-head shrink-0">
          <Button asChild variant="ghost" size="sm" className="-ml-2 h-7 gap-1.5 px-2">
            <Link to="/players">
              <ArrowLeft className="size-3.5" />
              <span className="stencil">Everyone</span>
            </Link>
          </Button>
        </header>

        {/* The name at the size the day number gets on the overview: this page
            is about one person and the page should say so from across a room. */}
        <section className="region shrink-0 border-b border-border px-4 py-5 md:px-6">
          <div className="flex flex-wrap items-center gap-3">
            <span
              aria-hidden
              className={cn(
                "size-2.5 shrink-0 rounded-full",
                player.online ? "bg-status-online" : "bg-status-unknown",
              )}
            />
            <h1 className="figure text-4xl leading-none">{player.name}</h1>
            {player.banned && <Badge variant="destructive">banned</Badge>}
            {isAdmin && (
              <Badge variant="outline" className="gap-1">
                <ShieldCheck className="size-3" />
                admin
              </Badge>
            )}
            {isWhitelisted && <Badge variant="outline">whitelisted</Badge>}
            <span className="stencil ml-auto">
              {player.online
                ? "online now"
                : player.lastOnline
                  ? `last seen ${formatAge((Date.now() - Date.parse(player.lastOnline)) / 1000)}`
                  : "never seen"}
            </span>
          </div>
        </section>

        {/*
          Live figures only exist while somebody is connected, so an offline
          player gets a dash rather than a zero pretending to be current.
        */}
        <section
          aria-label="Record"
          className="grid shrink-0 grid-cols-2 border-b border-border sm:grid-cols-4"
        >
          <Reading label="Level" value={offline ? "—" : String(player.level)} />
          <Reading label="Health" value={offline ? "—" : String(Math.round(player.health))} />
          <Reading label="Deaths" value={String(player.deaths)} />
          <Reading label="Zombies" value={String(player.zombieKills)} />
          <Reading label="Players killed" value={String(player.playerKills)} />
          <Reading label="Playtime" value={formatUptime(player.playTimeSeconds)} />
          <Reading label="Ping" value={offline ? "—" : `${player.ping}ms`} />
          <Reading
            label="Position"
            value={
              player.position
                ? `${Math.round(player.position.x)}, ${Math.round(player.position.z)}`
                : "—"
            }
          />
        </section>

        <Inventory player={player} />
        <LandClaims player={player} onRemove={run} />

        <section className="shrink-0 border-b border-border">
          <header className="region-head">
            <h2 className="stencil">Identity</h2>
          </header>
          <dl className="grid gap-x-8 gap-y-2 p-4 sm:grid-cols-2 md:px-6">
            <Fact label="Platform id" value={player.platformId} />
            {player.crossplatformId && <Fact label="Crossplay id" value={player.crossplatformId} />}
            <Fact label="Entity id" value={offline ? "—" : String(player.entityId)} />
            {player.ip && <Fact label="Address" value={player.ip} />}
          </dl>
        </section>

        <History player={player} />
      </div>

      <aside className="flex min-h-0 flex-col border-t border-border lg:border-t-0 lg:border-l lg:overflow-y-auto">
        <header className="region-head shrink-0">
          <h2 className="stencil">What you can do</h2>
        </header>

        {offline && (
          <p className="border-b border-border px-4 py-3 text-xs text-bone-faint md:px-6">
            Most of this needs them connected: the commands address somebody by entity id, and an
            offline player has none. Bans, admin levels and the whitelist work either way.
          </p>
        )}

        <Rail title="In the world">
          <Act
            label="Teleport"
            disabled={offline}
            onClick={() => setPending({ kind: "teleport", player })}
          />
          <Act label="Give things" disabled={offline} onClick={() => setGiving(true)} />
          <Act
            label="Give XP"
            disabled={offline}
            onClick={() => setPending({ kind: "xp", player })}
          />
          <Act
            label="Buff or debuff"
            disabled={offline}
            onClick={() => setPending({ kind: "buff", player })}
          />
          <Act
            label="Force containers open"
            disabled={offline}
            onClick={() => setPending({ kind: "unlock", player })}
          />
        </Rail>

        <Rail title="Say something">
          <PrivateMessage player={player} disabled={offline} />
        </Rail>

        <Rail title="Permissions">
          <div className="space-y-1">
            <div className="flex flex-wrap items-center gap-1">
              <span className="stencil mr-1">Grant</span>
              {ADMIN_LEVELS.map((rank) => (
                <Button
                  key={rank.level}
                  variant="ghost"
                  size="sm"
                  className="h-7 gap-1.5 px-2.5 text-xs"
                  title={rank.hint}
                  onClick={() =>
                    run(`${player.name} is now ${rank.label.toLowerCase()}`, () =>
                      api.setAdmin(serverId, player.platformId, rank.level),
                    )
                  }
                >
                  {rank.label}
                  <span className="readout text-2xs text-bone-faint">{rank.level}</span>
                </Button>
              ))}
            </div>
            <p className="text-2xs text-bone-faint">
              The number is the game's permission level, lower being more. Which commands each level
              may actually run is set per command in the server's serveradmin.xml, so these names
              are what the numbers are usually used for rather than fixed powers.
            </p>
          </div>
          {isAdmin && (
            <Act
              label="Demote to nobody"
              onClick={() =>
                run(`${player.name} is no longer an admin`, () =>
                  api.removeAdmin(serverId, player.platformId),
                )
              }
            />
          )}
          <Act
            label={isWhitelisted ? "Take off the whitelist" : "Add to the whitelist"}
            onClick={() =>
              isWhitelisted
                ? run(`${player.name} taken off the whitelist`, () =>
                    api.removeFromWhitelist(serverId, player.platformId),
                  )
                : run(`${player.name} added to the whitelist`, () =>
                    api.addToWhitelist(serverId, player.platformId),
                  )
            }
          />
          {!isWhitelisted && (
            <p className="text-xs text-ember">
              Once the whitelist has a single entry, nobody absent from it can join at all.
            </p>
          )}
        </Rail>

        {/*
          Moderation sits in reach, not at the far end of the rail.

          These three were pushed to the bottom together on the grounds that
          they are destructive. Kicking somebody is not: they can rejoin in ten
          seconds, and it is the most ordinary thing an operator does all
          evening. Holding it at arm's length made routine work awkward and the
          gap above it read as a layout fault rather than as caution.
        */}
        <Rail title="Moderate">
          <Act
            label="Kick"
            disabled={offline}
            onClick={() => setPending({ kind: "kick", player })}
          />
          <Act
            label="Kill"
            disabled={offline}
            onClick={() => setPending({ kind: "kill", player })}
          />
        </Rail>

        {/* The ban is the one that keeps somebody out, so it keeps its wall. */}
        <Rail title={player.banned ? "Banned" : "Ban"} tone="danger">
          {player.banned ? (
            <Act label="Lift the ban" onClick={() => setPending({ kind: "unban", player })} />
          ) : (
            <Act label="Ban them" onClick={() => setPending({ kind: "ban", player })} />
          )}
        </Rail>
      </aside>

      {giving && (
        <ItemPicker
          recipient={player.name}
          alsoOnline={online.length}
          onGive={giveBasket}
          onClose={() => setGiving(false)}
        />
      )}

      <PlayerActionDialog
        pending={pending}
        players={data?.players ?? []}
        onClose={() => setPending(null)}
        onRun={(done, fn) => {
          setPending(null);
          action.mutate(fn, {
            onSuccess: (result) => toast.success(done, { description: result.command }),
            onError: (error) =>
              toast.error("That did not work", {
                description: error instanceof ApiError ? error.message : "The action failed.",
              }),
          });
        }}
      />
    </div>
  );
}

/** One figure in the record strip, matching the overview's readings. */
function Reading({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-baseline gap-2 border-border px-4 py-3 sm:border-l sm:first:border-l-0 md:px-6 [&:nth-child(n+3)]:border-t sm:[&:nth-child(n+5)]:border-t">
      <span className="stencil">{label}</span>
      <span className="figure ml-auto text-2xl">{value}</span>
    </div>
  );
}

function Fact({ label, value }: { label: string; value: string }) {
  return (
    <div className="min-w-0">
      <dt className="stencil">{label}</dt>
      <dd className="readout mt-0.5 truncate text-xs text-bone-dim">{value}</dd>
    </div>
  );
}

/** A block of the right-hand rail. */
function Rail({ title, children, tone }: { title: string; children: ReactNode; tone?: "danger" }) {
  return (
    <section className={cn("border-b border-border", tone === "danger" && "border-t")}>
      <header className="region-head">
        <h3 className={cn("stencil", tone === "danger" && "text-crimson-lit")}>{title}</h3>
      </header>
      <div className="flex flex-col items-start gap-1 p-3 md:px-4">{children}</div>
    </section>
  );
}

function Act({
  label,
  onClick,
  disabled,
}: {
  label: string;
  onClick: () => void;
  disabled?: boolean;
}) {
  return (
    <Button
      variant="ghost"
      size="sm"
      disabled={disabled}
      className="h-7 w-full justify-start px-2 text-bone"
      onClick={onClick}
    >
      {label}
    </Button>
  );
}

/** A message to this player rather than to the whole server. */
function PrivateMessage({ player, disabled }: { player: Player; disabled: boolean }) {
  const serverId = useServerId();
  const [message, setMessage] = useState("");

  const send = useMutation({
    mutationFn: () => api.privateMessage(serverId, player.entityId, message.trim()),
    onSuccess: (result) => {
      toast.success(`Sent to ${player.name}`, { description: result.command });
      setMessage("");
    },
    onError: (error) =>
      toast.error("Not sent", {
        description: error instanceof ApiError ? error.message : "The message failed.",
      }),
  });

  return (
    <div className="flex w-full flex-wrap items-center gap-2">
      <Input
        value={message}
        onChange={(e) => setMessage(e.target.value)}
        placeholder={disabled ? "Not connected" : "Only they see it"}
        className="h-8 flex-1 text-sm"
        maxLength={200}
        disabled={disabled || send.isPending}
        onKeyDown={(e) => {
          if (e.key === "Enter" && message.trim() !== "") send.mutate();
        }}
      />
      <Button
        size="sm"
        className="h-8"
        disabled={disabled || send.isPending || message.trim() === ""}
        onClick={() => send.mutate()}
      >
        Send
      </Button>
    </div>
  );
}

/**
 * What they are carrying.
 *
 * Fetched on request rather than with the page: it is a console round trip per
 * look, and most visits here are about something else.
 */
function Inventory({ player }: { player: Player }) {
  const serverId = useServerId();
  const [open, setOpen] = useState(false);
  const { data, isLoading, error } = useQuery({
    queryKey: ["inventory", serverId, player.entityId],
    queryFn: () => api.inventory(serverId, player.entityId),
    enabled: open,
  });

  return (
    <section className="border-b border-border">
      <header className="region-head">
        <h2 className="stencil">Inventory</h2>
        <Button
          variant="ghost"
          size="sm"
          disabled={!player.online}
          className="-my-1 h-6 px-2 text-xs"
          onClick={() => setOpen((v) => !v)}
        >
          {open ? "Hide" : "Show"}
        </Button>
        <span className="text-xs text-bone-faint">Kept only after about thirty seconds online</span>
      </header>
      {open && (
        <div className="p-4 md:px-6">
          {isLoading ? (
            <Skeleton className="h-32 w-full rounded-none" />
          ) : error ? (
            <p className="text-xs text-crimson-lit">
              {error instanceof ApiError ? error.message : "The panel could not ask."}
            </p>
          ) : (
            <pre className="readout max-h-80 overflow-auto text-xs leading-relaxed whitespace-pre-wrap text-bone-dim">
              {data?.text.trim() || "The server said nothing."}
            </pre>
          )}
        </div>
      )}
    </section>
  );
}

/** The ground this player has claimed, which a chunk reset will not touch. */
function LandClaims({
  player,
  onRemove,
}: {
  player: Player;
  onRemove: (done: string, run: () => Promise<ActionResult>) => void;
}) {
  const serverId = useServerId();
  const [open, setOpen] = useState(false);
  const { data, isLoading } = useQuery({
    queryKey: ["land-claims", serverId, player.platformId],
    queryFn: () => api.landClaims(serverId, player.platformId),
    enabled: open,
  });

  return (
    <section className="border-b border-border">
      <header className="region-head">
        <h2 className="stencil">Land claims</h2>
        <Button
          variant="ghost"
          size="sm"
          className="-my-1 h-6 px-2 text-xs"
          onClick={() => setOpen((v) => !v)}
        >
          {open ? "Hide" : "Show"}
        </Button>
        <Button
          variant="ghost"
          size="sm"
          className="-my-1 ml-auto h-6 px-2 text-xs text-crimson-lit"
          onClick={() =>
            onRemove(`${player.name}'s claims released`, () =>
              api.removeLandClaims(serverId, player.platformId),
            )
          }
        >
          Release them
        </Button>
      </header>
      {open && (
        <div className="space-y-2 p-4 md:px-6">
          {isLoading ? (
            <Skeleton className="h-24 w-full rounded-none" />
          ) : (
            <pre className="readout max-h-64 overflow-auto text-xs leading-relaxed whitespace-pre-wrap text-bone-dim">
              {data?.text.trim() || "The server said nothing."}
            </pre>
          )}
          <p className="max-w-prose text-xs text-bone-faint">
            Releasing leaves the blocks standing but stops them protecting anything, which puts that
            ground back in reach of a chunk reset.
          </p>
        </div>
      )}
    </section>
  );
}

/**
 * What this player has been doing, out of the live feed.
 *
 * The panel already streams every line the server logs and works out who each
 * one belongs to, so the page can answer "what happened with them" without
 * another request. It is also what the bottom of this column is for: a record
 * that stops at a list of identifiers has not said much about a person.
 *
 * Only as far back as this session, because the feed is a ring buffer in memory
 * and not a history the panel keeps — so it says so, rather than implying
 * somebody has been quiet.
 */
function History({ player }: { player: Player }) {
  const { events } = useEvents();
  const theirs = events
    .filter((event) => event.player === player.name)
    .slice(-60)
    .reverse();

  return (
    <section className="flex min-h-0 flex-1 flex-col border-b border-border">
      <header className="region-head shrink-0">
        <h2 className="stencil">Since the panel started</h2>
        <span className="readout text-xs text-bone-faint">{theirs.length}</span>
      </header>
      {theirs.length === 0 ? (
        <p className="p-4 text-xs text-bone-faint md:px-6">
          Nothing from them in the feed yet. It only reaches back as far as this panel has been
          running.
        </p>
      ) : (
        <ol className="min-h-0 flex-1 overflow-y-auto">
          {theirs.map((event) => (
            <li
              key={event.seq}
              className="flex items-baseline gap-3 border-b border-border/60 px-4 py-1.5 last:border-b-0 md:px-6"
            >
              <span className="readout shrink-0 text-xs text-bone-faint">
                {new Date(event.at).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}
              </span>
              <span className="stencil shrink-0 text-bone-faint">{event.kind}</span>
              <span className="min-w-0 flex-1 truncate text-xs text-bone-dim">{event.message}</span>
            </li>
          ))}
        </ol>
      )}
    </section>
  );
}

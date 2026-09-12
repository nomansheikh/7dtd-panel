import { useState, type ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { AlertTriangle, ArrowUpRight, Check, Copy } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { useDashboard } from "@/hooks/use-dashboard";
import { useEvents } from "@/hooks/use-events";
import { usePlayers } from "@/hooks/use-players";
import { useServerId } from "@/hooks/use-servers";
import { bloodMoonProgress, formatGameClock, formatUptime } from "@/lib/format";
import { cn } from "@/lib/utils";
import { api, type Dashboard, type EventKind, type PanelEvent, type Player } from "@/lib/api";

/**
 * The overview, laid out as a deck rather than a stack.
 *
 * Two columns that fill the viewport: the world and the people in it on the
 * left, the live feed running full height on the right. Nothing scrolls the
 * page — the roster and the feed scroll inside themselves — so the shape of
 * the screen is the same whether one person is online or thirty.
 */
export function DashboardPage() {
  const { data, error, isLoading } = useDashboard();

  if (isLoading && !data) {
    return (
      <div className="grid h-full grid-cols-1 gap-px lg:grid-cols-[1fr_22rem]">
        <Skeleton className="h-full w-full rounded-none" />
        <Skeleton className="hidden h-full w-full rounded-none lg:block" />
      </div>
    );
  }

  if (!data) {
    return (
      <div className="flex h-full items-center justify-center p-8">
        <div className="max-w-md text-center">
          <AlertTriangle className="mx-auto size-6 text-crimson-lit" />
          <h2 className="mt-3 font-display text-lg tracking-wider uppercase">
            Could not load the overview
          </h2>
          <p className="mt-2 text-sm text-bone-dim">
            {error instanceof Error ? error.message : "The panel did not answer."}
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="grid h-full min-h-0 grid-rows-[auto_1fr] overflow-y-auto lg:grid-cols-[1fr_22rem] lg:grid-rows-1 lg:overflow-hidden">
      <div className="flex min-h-0 min-w-0 flex-col lg:overflow-hidden">
        <Banner data={data} />
        <Clock data={data} />
        <Readings data={data} />
        <Vitals />
        <Roster max={data.players.max} />
        <Joining server={data.server} />
      </div>

      <aside className="flex min-h-0 flex-col border-t border-border lg:border-t-0 lg:border-l">
        <LiveFeed />
      </aside>
    </div>
  );
}

/** The game server is unreachable but the panel is fine. Say which. */
function Banner({ data }: { data: Dashboard }) {
  if (data.status !== "offline") return null;
  return (
    <div className="shrink-0 border-b border-crimson/40 bg-crimson/12 px-4 py-3 md:px-6">
      <p className="flex flex-wrap items-center gap-x-2 text-sm">
        <AlertTriangle className="size-4 shrink-0 text-crimson-lit" />
        <span className="font-medium">The game server is not responding.</span>
        <span className="text-bone-dim">
          Everything below is the last reading before it stopped answering.
        </span>
      </p>
      {data.lastError && (
        <p className="readout mt-1 pl-6 text-xs break-all text-bone-faint">{data.lastError}</p>
      )}
    </div>
  );
}

/**
 * The day, at the size the day deserves.
 *
 * It is the largest thing in the panel by a wide margin, set flush left and
 * cropped tight against the rules, because it is the one number an operator
 * reads a hundred times a session.
 */
function Clock({ data }: { data: Dashboard }) {
  const { world, server, bloodMoon } = data;

  return (
    <section
      aria-label="World clock"
      className="panel-moon relative flex shrink-0 flex-wrap items-end justify-between gap-x-8 gap-y-4 border-b border-border px-4 pt-7 pb-5 md:px-6"
    >
      <div className="min-w-0">
        <div className="flex items-baseline gap-x-5">
          <span className="figure text-6xl leading-[0.78] sm:text-7xl">{world.day}</span>
          <div className="min-w-0">
            <div className="stencil">Day</div>
            <div className="readout mt-1.5 text-3xl leading-none text-bone-dim">
              {formatGameClock(world.hour, world.minute)}
            </div>
          </div>
        </div>
        <dl className="mt-4 flex flex-wrap gap-x-6 gap-y-1 text-xs">
          <Fact label="World" value={world.name} />
          <Fact label="Mode" value={server.gameMode} />
          <Fact label="Version" value={server.version} mono />
          {server.region && <Fact label="Region" value={server.region} />}
        </dl>
      </div>

      {bloodMoon && <Countdown world={world} bloodMoon={bloodMoon} />}
    </section>
  );
}

function Fact({ label, value, mono }: { label: string; value?: string; mono?: boolean }) {
  if (!value) return null;
  return (
    <div className="flex items-baseline gap-1.5">
      <dt className="stencil">{label}</dt>
      <dd className={cn("text-bone-dim", mono && "readout")}>{value}</dd>
    </div>
  );
}

function Countdown({
  world,
  bloodMoon,
}: {
  world: Dashboard["world"];
  bloodMoon: NonNullable<Dashboard["bloodMoon"]>;
}) {
  const { daysAway } = bloodMoonProgress(world.day, bloodMoon.nextDay);
  const imminent = daysAway <= 1;

  if (bloodMoon.active) {
    return (
      <div className="text-right">
        <div className="stencil text-crimson-lit">Blood moon</div>
        <p className="figure animate-breathe mt-2 text-5xl text-crimson-lit">TONIGHT</p>
        <p className="mt-1.5 text-xs text-bone-dim">They are already coming.</p>
      </div>
    );
  }

  return (
    <div className="text-right">
      <div className={cn("stencil", imminent && "text-crimson-lit")}>Blood moon</div>
      <p className="mt-2 flex items-baseline justify-end gap-2">
        <span className={cn("figure text-5xl", imminent && "text-crimson-lit")}>{daysAway}</span>
        <span className="font-display text-sm font-semibold tracking-[0.14em] text-bone-dim uppercase">
          {daysAway === 1 ? "day" : "days"}
        </span>
      </p>
      <p className="readout mt-1.5 text-xs text-bone-faint">
        Day {bloodMoon.nextDay} · {String(bloodMoon.nextHour).padStart(2, "0")}:00
      </p>
    </div>
  );
}

/** Four numbers on one rule, not four cards. */
function Readings({ data }: { data: Dashboard }) {
  const { world, players, uptime } = data;
  // The panel extrapolates uptime server-side and freezes it when the server
  // stops answering, so this just renders what it is given.
  const seconds = uptime ? uptime.seconds : null;

  return (
    <section
      aria-label="Current readings"
      className="grid shrink-0 grid-cols-2 border-b border-border sm:grid-cols-4"
    >
      <Reading label="Players" value={`${players.online}/${players.max || "?"}`} />
      <Reading
        label="Hostiles"
        value={String(world.hostiles)}
        tone={world.hostiles > 0 ? "warn" : undefined}
      />
      <Reading label="Animals" value={String(world.animals)} />
      <Reading label="Uptime" value={seconds === null ? "—" : formatUptime(seconds)} />
    </section>
  );
}

/**
 * How hard the server is actually working.
 *
 * None of this is in the REST API — it comes back from the mem and version
 * commands — which is presumably why no panel shows it. It is also the first
 * thing worth knowing when players say the world feels slow: a server at five
 * frames a second and one at twenty look identical from every other screen
 * here.
 */
function Vitals() {
  const serverId = useServerId();
  const { data } = useQuery({
    queryKey: ["vitals", serverId],
    queryFn: () => api.vitals(serverId),
    enabled: serverId !== "",
    refetchInterval: 20_000,
    retry: 1,
  });

  if (!data) return null;

  // Twenty is the rate a dedicated server aims for, so the warning sits well
  // below it rather than at anything short of perfect.
  const slow = data.fps > 0 && data.fps < 12;
  const heapShare = data.maxHeapMb > 0 ? data.heapMb / data.maxHeapMb : 0;

  return (
    <section
      aria-label="Server vitals"
      className="grid shrink-0 grid-cols-2 border-b border-border sm:grid-cols-4"
    >
      <Reading
        label="Server FPS"
        value={data.fps > 0 ? data.fps.toFixed(0) : "—"}
        tone={slow ? "warn" : undefined}
      />
      <Reading
        label="Heap"
        value={`${Math.round(data.heapMb)}M`}
        tone={heapShare > 0.85 ? "warn" : undefined}
      />
      <Reading label="Memory" value={`${(data.rssMb / 1024).toFixed(1)}G`} />
      <Reading label="Chunks" value={String(data.chunks)} />
      {/* The build and its mods, which is the other half of "what am I even
          running" and the thing a bug report always asks for. */}
      <div className="col-span-2 flex flex-wrap items-baseline gap-x-3 gap-y-1 border-t border-border px-4 py-2.5 sm:col-span-4 md:px-6">
        <span className="stencil">Running</span>
        <span className="readout text-xs text-bone-dim">{data.gameVersion || "unknown"}</span>
        {data.mods.length > 0 && (
          <span className="readout text-xs text-bone-faint">
            {data.mods.map((m) => `${m.name} ${m.version}`).join("  ·  ")}
          </span>
        )}
      </div>
    </section>
  );
}

function Reading({ label, value, tone }: { label: string; value: string; tone?: "warn" }) {
  return (
    <div className="flex items-baseline gap-2 border-border px-4 py-3 sm:border-l sm:first:border-l-0 md:px-6 [&:nth-child(n+3)]:border-t sm:[&:nth-child(n+3)]:border-t-0">
      <span className="stencil">{label}</span>
      <span className={cn("figure ml-auto text-2xl", tone === "warn" && "text-ember")}>
        {value}
      </span>
    </div>
  );
}

/**
 * Everyone the server knows, online first.
 *
 * Not just the people currently connected: one player online would otherwise
 * leave most of this column empty, and who was here yesterday is what an
 * operator wants next anyway. Offline rows are dimmed and carry what the
 * server still remembers about them.
 */
function Roster({ max }: { max: number }) {
  const { data, isLoading } = usePlayers();
  const players = data?.players ?? [];
  const online = players.filter((p) => p.online).length;

  return (
    <section
      aria-label="Players"
      className="region flex min-h-0 flex-1 flex-col border-b border-border"
    >
      <Head
        title="Players"
        count={`${online}/${max || "?"} online · ${players.length} known`}
        to="/players"
      />

      {isLoading && !data ? (
        <div className="space-y-px p-4">
          <Skeleton className="h-9 w-full rounded-none" />
          <Skeleton className="h-9 w-full rounded-none" />
        </div>
      ) : players.length === 0 ? (
        <Empty>Nobody has joined this server yet.</Empty>
      ) : (
        <ul className="min-h-0 flex-1 divide-y divide-border overflow-y-auto">
          {players.map((player) => (
            <PlayerRow key={player.platformId || player.entityId} player={player} />
          ))}
        </ul>
      )}
    </section>
  );
}

function PlayerRow({ player }: { player: Player }) {
  // Health is the one player stat worth a glance from the overview: a survivor
  // at 12 health is about to become a death in the feed alongside.
  const hurt = player.health < 35;

  return (
    <li className={cn("flex items-center gap-3 px-4 py-2 md:px-6", !player.online && "opacity-55")}>
      <span
        aria-hidden
        className={cn(
          "size-1.5 shrink-0",
          player.online ? "bg-status-online" : "bg-bone/25",
          player.banned && "bg-crimson-lit",
        )}
      />
      <span className="w-36 shrink-0 truncate text-sm font-medium">{player.name}</span>

      {player.online ? (
        <>
          <span className="stencil w-12 shrink-0">Lv {player.level}</span>
          {player.health <= 0 ? (
            // An empty bar and a dead player look identical, and they are not
            // the same thing. Say it.
            <span className="stencil w-20 shrink-0 text-crimson-lit">Dead</span>
          ) : (
            /* A bar rather than a number: three digits of health tell you less
               at a glance than a bar that is nearly empty. */
            <span
              className="hidden h-1 w-20 shrink-0 bg-bone/15 sm:block"
              title={`${player.health} health`}
            >
              <span
                className={cn("block h-full", hurt ? "bg-crimson-lit" : "bg-status-online")}
                style={{ width: `${Math.min(100, player.health)}%` }}
              />
            </span>
          )}
          <span className="readout ml-auto hidden text-xs text-bone-faint md:inline">
            {player.zombieKills} kills · {player.deaths} deaths
          </span>
          <span className="readout w-14 shrink-0 text-right text-xs text-bone-faint">
            {player.ping}ms
          </span>
        </>
      ) : (
        <>
          <span className="stencil">{player.banned ? "Banned" : "Offline"}</span>
          <span className="readout ml-auto text-xs text-bone-faint">
            {formatUptime(player.playTimeSeconds)} played
          </span>
          <span className="readout w-14 shrink-0 text-right text-xs text-bone-faint">
            {lastSeen(player)}
          </span>
        </>
      )}
    </li>
  );
}

/** How long ago somebody was last on, in the shortest honest form. */
function lastSeen(player: Player): string {
  if (!player.lastOnline) return "—";
  const at = Date.parse(player.lastOnline);
  if (Number.isNaN(at)) return "—";
  const minutes = Math.max(0, Math.round((Date.now() - at) / 60_000));
  if (minutes < 60) return `${minutes}m`;
  if (minutes < 1440) return `${Math.round(minutes / 60)}h`;
  return `${Math.round(minutes / 1440)}d`;
}

/**
 * How to actually join.
 *
 * The address is the one the game server advertises for itself, which is not
 * necessarily how the panel reaches it: the panel may sit on the same LAN while
 * players connect from outside.
 */
function Joining({ server }: { server: Dashboard["server"] }) {
  const [copied, setCopied] = useState(false);

  async function copy(text: string) {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      // Clipboard access is denied outside a secure context, which a LAN panel
      // on plain HTTP often is. The address is on screen to copy by hand.
    }
  }

  return (
    <section
      aria-label="Connection"
      className="flex shrink-0 flex-wrap items-center gap-x-4 gap-y-2 px-4 py-3 md:px-6"
    >
      <span className="stencil">Join at</span>
      {server.connect ? (
        <>
          <code className="readout text-sm text-bone">{server.connect}</code>
          <Button
            variant="ghost"
            size="icon"
            className="size-7 text-bone-faint hover:text-foreground"
            onClick={() => void copy(server.connect!)}
          >
            {copied ? <Check className="text-status-online" /> : <Copy />}
            <span className="sr-only">Copy the connect address</span>
          </Button>
        </>
      ) : (
        <span className="text-sm text-bone-faint">Not advertised yet.</span>
      )}
      <span className="readout ml-auto text-xs text-bone-faint">
        panel → {server.panelUrl.replace(/^https?:\/\//, "")}
      </span>
    </section>
  );
}

const FEED_TONE: Partial<Record<EventKind, string>> = {
  chat: "text-foreground",
  join: "text-status-online",
  leave: "text-bone-dim",
  death: "text-crimson-lit",
  status: "text-ember",
};

const FEED_LABEL: Partial<Record<EventKind, string>> = {
  chat: "say",
  join: "in",
  leave: "out",
  death: "died",
  status: "panel",
};

/**
 * The live feed, full height down the right.
 *
 * Trimmed to what happened to people: the events page carries every log line,
 * and raw engine noise on the overview would bury the one death that mattered.
 */
function LiveFeed() {
  const { events } = useEvents();
  const notable = events
    .filter((e) => e.kind !== "log")
    .sort((a, b) => Date.parse(b.at) - Date.parse(a.at))
    .slice(0, 60);

  return (
    <section aria-label="Live feed" className="region flex min-h-0 flex-1 flex-col">
      <Head title="Live" to="/events" />
      {notable.length === 0 ? (
        <Empty>Quiet since the panel connected.</Empty>
      ) : (
        <ul className="min-h-0 flex-1 divide-y divide-border overflow-y-auto">
          {notable.map((event) => (
            <FeedRow key={event.seq} event={event} />
          ))}
        </ul>
      )}
    </section>
  );
}

function FeedRow({ event }: { event: PanelEvent }) {
  const when = new Date(event.at);
  const time = Number.isNaN(when.getTime())
    ? "--:--"
    : when.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", hour12: false });

  return (
    <li className="px-4 py-2">
      <div className="flex items-baseline gap-2">
        <span className="readout shrink-0 text-xs text-bone-faint">{time}</span>
        {FEED_LABEL[event.kind] && (
          <span className="stencil shrink-0 text-2xs">{FEED_LABEL[event.kind]}</span>
        )}
      </div>
      <p className={cn("mt-0.5 text-sm leading-snug break-words", FEED_TONE[event.kind])}>
        {event.player && <span className="font-medium">{event.player}: </span>}
        {event.message}
      </p>
    </li>
  );
}

function Head({ title, count, to }: { title: string; count?: string; to: string }) {
  return (
    <header className="region-head shrink-0">
      <h2 className="stencil">{title}</h2>
      {count && <span className="readout text-xs text-bone-dim">{count}</span>}
      <Button
        asChild
        variant="ghost"
        size="sm"
        className="ml-auto h-5 px-1.5 text-xs text-bone-faint hover:text-foreground"
      >
        <Link to={to}>
          All <ArrowUpRight className="size-3" />
        </Link>
      </Button>
    </header>
  );
}

function Empty({ children }: { children: ReactNode }) {
  return (
    <p className="flex flex-1 items-center justify-center p-6 text-center text-sm text-bone-faint">
      {children}
    </p>
  );
}

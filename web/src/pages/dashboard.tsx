import { useState } from "react";
import { Link } from "react-router-dom";
import { AlertTriangle, ArrowUpRight, Check, Copy, Moon, Skull } from "lucide-react";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { HordeMeter } from "@/components/horde-meter";
import { useDashboard } from "@/hooks/use-dashboard";
import { useEvents } from "@/hooks/use-events";
import { usePlayers } from "@/hooks/use-players";
import { bloodMoonProgress, formatGameClock, formatUptime } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { Dashboard, EventKind, PanelEvent, Player } from "@/lib/api";

export function DashboardPage() {
  const { data, error, isLoading } = useDashboard();

  if (isLoading && !data) {
    return (
      <div className="space-y-5">
        <Skeleton className="h-56 w-full" />
        <Skeleton className="h-24 w-full" />
        <Skeleton className="h-72 w-full" />
      </div>
    );
  }

  if (!data) {
    return (
      <Alert variant="destructive">
        <AlertTriangle />
        <AlertTitle>Could not load the overview</AlertTitle>
        <AlertDescription>
          {error instanceof Error ? error.message : "The panel did not answer."}
        </AlertDescription>
      </Alert>
    );
  }

  const { status, lastError } = data;

  return (
    <div className="space-y-5">
      {/* The game server is unreachable but the panel is fine. Say which. */}
      {status === "offline" && (
        <Alert variant="destructive" className="animate-rise">
          <AlertTriangle />
          <AlertTitle>The game server is not responding</AlertTitle>
          <AlertDescription>
            <span>Everything below is the last reading before it stopped answering.</span>
            {lastError && (
              <span className="mt-1 block font-mono text-xs break-all">{lastError}</span>
            )}
          </AlertDescription>
        </Alert>
      )}

      {/* Each block rises a beat after the one above it, so the page arrives
          rather than appearing. */}
      <Stagger index={0}>
        <Hero data={data} />
      </Stagger>
      <Stagger index={1}>
        <Readings data={data} />
      </Stagger>
      <div className="grid gap-5 lg:grid-cols-[1.15fr_1fr]">
        <Stagger index={2}>
          <OnlinePlayers max={data.players.max} />
        </Stagger>
        <Stagger index={3}>
          <LiveFeed />
        </Stagger>
      </div>
      <Stagger index={4}>
        <Joining server={data.server} />
      </Stagger>
    </div>
  );
}

/** One step of the page-load cascade. */
function Stagger({ index, children }: { index: number; children: React.ReactNode }) {
  return (
    <div className="animate-rise" style={{ animationDelay: `${index * 70}ms` }}>
      {children}
    </div>
  );
}

/**
 * The hero: the seven-day cycle, which is the only thing this game is about.
 *
 * The day and the countdown share one panel because they are one fact read two
 * ways, and the panel itself bleeds crimson in proportion to how close the
 * horde is — the same signal the whole interface is keyed to, at its loudest
 * here.
 */
function Hero({ data }: { data: Dashboard }) {
  const { world, server, bloodMoon } = data;

  return (
    <section
      aria-label="World"
      className="panel-moon relative overflow-hidden rounded-md px-6 py-6 md:px-8"
    >
      <div className="flex flex-wrap items-end justify-between gap-x-10 gap-y-6">
        <div>
          <div className="flex items-center gap-2">
            <span className="stencil">{world.name || "World"}</span>
            {server.gameMode && (
              <Badge
                variant="outline"
                className="border-border/60 text-[10px] tracking-wider uppercase"
              >
                {server.gameMode}
              </Badge>
            )}
          </div>
          <div className="mt-3 flex flex-wrap items-baseline gap-x-5">
            <span className="figure text-7xl sm:text-8xl">
              <span className="text-bone-dim">DAY</span> {world.day}
            </span>
            <span className="readout text-3xl text-bone-dim sm:text-4xl">
              {formatGameClock(world.hour, world.minute)}
            </span>
          </div>
        </div>

        {bloodMoon ? (
          <Countdown
            currentDay={world.day}
            nextDay={bloodMoon.nextDay}
            nextHour={bloodMoon.nextHour}
            active={bloodMoon.active}
          />
        ) : (
          <p className="text-sm text-bone-faint">Waiting for the first reading.</p>
        )}
      </div>

      {bloodMoon && (
        <HordeMeter
          className="mt-7"
          currentDay={world.day}
          nextDay={bloodMoon.nextDay}
          active={bloodMoon.active}
        />
      )}
    </section>
  );
}

function Countdown({
  currentDay,
  nextDay,
  nextHour,
  active,
}: {
  currentDay: number;
  nextDay: number;
  nextHour: number;
  active: boolean;
}) {
  const { daysAway } = bloodMoonProgress(currentDay, nextDay);
  const imminent = daysAway <= 1;

  if (active) {
    return (
      <div className="text-right">
        <div className="flex items-center justify-end gap-2">
          <Moon className="size-4 animate-breathe text-crimson-lit" />
          <span className="stencil text-crimson-lit">Blood moon</span>
        </div>
        <p className="figure mt-2 animate-breathe text-5xl text-crimson-lit sm:text-6xl">TONIGHT</p>
        <p className="mt-2 text-sm text-bone-dim">They are already coming.</p>
      </div>
    );
  }

  return (
    <div className="text-right">
      <div className="flex items-center justify-end gap-2">
        <Moon className={cn("size-4", imminent ? "text-crimson-lit" : "text-bone-faint")} />
        <span className={cn("stencil", imminent && "text-crimson-lit")}>Blood moon</span>
      </div>
      <p className="mt-2 flex items-baseline justify-end gap-2.5">
        <span className={cn("figure text-6xl sm:text-7xl", imminent && "text-crimson-lit")}>
          {daysAway}
        </span>
        <span className="font-display text-lg font-semibold tracking-[0.14em] text-bone-dim uppercase">
          {daysAway === 1 ? "day" : "days"}
        </span>
      </p>
      <p className="readout mt-2 text-sm text-bone-faint">
        Day {nextDay} at {String(nextHour).padStart(2, "0")}:00
      </p>
    </div>
  );
}

/** The four numbers, in one cut strip rather than four floating cards. */
function Readings({ data }: { data: Dashboard }) {
  const { world, players, uptime } = data;
  // The panel already extrapolates uptime server-side and freezes it when the
  // server stops answering, so this just renders what it is given.
  const seconds = uptime ? uptime.seconds : null;

  return (
    <section
      aria-label="Current readings"
      className="panel grid grid-cols-2 rounded-md sm:grid-cols-4"
    >
      <Reading label="Players" value={`${players.online} / ${players.max || "?"}`} />
      <Reading
        label="Hostiles"
        value={String(world.hostiles)}
        tone={world.hostiles > 0 ? "warn" : undefined}
      />
      <Reading label="Animals" value={String(world.animals)} />
      <Reading
        label="Uptime"
        value={seconds === null ? "—" : formatUptime(seconds)}
        hint={seconds === null ? "not sampled yet" : undefined}
      />
    </section>
  );
}

function Reading({
  label,
  value,
  hint,
  tone,
}: {
  label: string;
  value: string;
  hint?: string;
  tone?: "warn";
}) {
  return (
    <div className="border-border px-6 py-4 [&:nth-child(n+3)]:border-t sm:border-l sm:first:border-l-0 sm:[&:nth-child(n+3)]:border-t-0">
      <div className="stencil">{label}</div>
      <div className={cn("figure mt-2 text-3xl", tone === "warn" && "text-ember")}>{value}</div>
      {hint && <div className="mt-1 text-xs text-bone-faint">{hint}</div>}
    </div>
  );
}

/** Who is actually on the server, which is the first thing anyone opens this for. */
function OnlinePlayers({ max }: { max: number }) {
  const { data, isLoading } = usePlayers();
  const online = (data?.players ?? []).filter((p) => p.online);

  return (
    <section aria-label="Online players" className="panel flex h-full flex-col rounded-md">
      <PanelHeading
        title="On the server"
        count={`${online.length} / ${max || "?"}`}
        to="/players"
      />

      {isLoading && !data ? (
        <div className="space-y-2 p-4">
          <Skeleton className="h-10 w-full" />
          <Skeleton className="h-10 w-full" />
        </div>
      ) : online.length === 0 ? (
        <Empty>Nobody is on the server.</Empty>
      ) : (
        <ul className="divide-y divide-border">
          {online.map((player) => (
            <PlayerRow key={player.entityId} player={player} />
          ))}
        </ul>
      )}
    </section>
  );
}

function PlayerRow({ player }: { player: Player }) {
  // Health is the one player stat worth a glance from the overview: a survivor
  // at 12 health is about to become a death in the feed below.
  const hurt = player.health > 0 && player.health < 35;

  return (
    <li className="flex items-center gap-3 px-5 py-3">
      <span
        aria-hidden
        className="grid size-8 shrink-0 place-items-center rounded-sm bg-ash-raised font-display text-sm font-bold text-bone-dim"
      >
        {player.name.slice(0, 2).toUpperCase()}
      </span>

      <div className="min-w-0 flex-1">
        <div className="truncate text-sm font-medium">{player.name}</div>
        <div className="readout mt-0.5 text-xs text-bone-faint">
          Level {player.level} · {player.zombieKills} zombies · {player.deaths} deaths
        </div>
      </div>

      <Tooltip>
        <TooltipTrigger asChild>
          <div className="flex items-center gap-1.5">
            <Skull className={cn("size-3.5", hurt ? "text-crimson-lit" : "text-bone-faint")} />
            <span className={cn("readout text-sm", hurt ? "text-crimson-lit" : "text-bone-dim")}>
              {player.health}
            </span>
          </div>
        </TooltipTrigger>
        <TooltipContent>Health</TooltipContent>
      </Tooltip>

      <span className="readout w-14 text-right text-xs text-bone-faint">{player.ping} ms</span>
    </li>
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
  chat: "chat",
  join: "join",
  leave: "left",
  death: "died",
  status: "panel",
};

/**
 * The live feed, trimmed to the events a person cares about.
 *
 * The events page shows every log line; this shows what happened to people.
 * Raw log noise on the overview would bury the one death that mattered.
 */
function LiveFeed() {
  const { events } = useEvents();

  const notable = events
    .filter((e) => e.kind !== "log")
    .sort((a, b) => Date.parse(b.at) - Date.parse(a.at))
    .slice(0, 9);

  return (
    <section aria-label="Live feed" className="panel flex h-full flex-col rounded-md">
      <PanelHeading title="Live" to="/events" />

      {notable.length === 0 ? (
        <Empty>Nothing has happened since the panel connected.</Empty>
      ) : (
        <ul className="divide-y divide-border">
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
    <li className="flex items-baseline gap-3 px-5 py-2.5">
      <span className="readout shrink-0 text-xs text-bone-faint">{time}</span>
      {FEED_LABEL[event.kind] && (
        <span className="stencil w-11 shrink-0 text-[10px] tracking-[0.1em]">
          {FEED_LABEL[event.kind]}
        </span>
      )}
      <span className={cn("min-w-0 flex-1 truncate text-sm", FEED_TONE[event.kind])}>
        {event.player && <span className="font-medium">{event.player}: </span>}
        {event.message}
      </span>
    </li>
  );
}

function PanelHeading({ title, count, to }: { title: string; count?: string; to: string }) {
  return (
    <header className="flex items-center gap-3 border-b border-border px-5 py-3">
      <h2 className="stencil">{title}</h2>
      {count && <span className="readout text-xs text-bone-dim">{count}</span>}
      <Button
        asChild
        variant="ghost"
        size="sm"
        className="ml-auto h-6 px-2 text-xs text-bone-faint hover:text-foreground"
      >
        <Link to={to}>
          All <ArrowUpRight className="size-3" />
        </Link>
      </Button>
    </header>
  );
}

function Empty({ children }: { children: React.ReactNode }) {
  return <p className="flex-1 px-5 py-10 text-center text-sm text-bone-faint">{children}</p>;
}

/**
 * How to actually join this server.
 *
 * The address is the one the game server advertises for itself, which is not
 * necessarily how the panel reaches it: the panel may sit on the same LAN while
 * players connect from outside. Both are shown, labelled, rather than implying
 * they are interchangeable.
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
    <section aria-label="Connection" className="panel rounded-md px-6 py-5">
      <div className="flex flex-wrap items-center justify-between gap-x-8 gap-y-4">
        <div className="min-w-0">
          <h2 className="stencil">Joining</h2>
          {server.description && <p className="mt-2 text-sm text-bone-dim">{server.description}</p>}
          <p className="mt-2 max-w-prose text-xs text-bone-faint">
            In game: Join a Game, then Connect to IP. The address here is the one the server
            advertises; on the same network you may need its local address instead.
          </p>
        </div>

        <div className="flex flex-col items-start gap-2">
          {server.connect ? (
            <div className="flex items-center gap-2">
              <code className="readout rounded-sm border border-border bg-background px-3 py-2 text-base">
                {server.connect}
              </code>
              <Button variant="outline" size="icon" onClick={() => void copy(server.connect!)}>
                {copied ? <Check className="text-status-online" /> : <Copy />}
                <span className="sr-only">Copy the connect address</span>
              </Button>
            </div>
          ) : (
            <p className="text-sm text-bone-faint">The server has not reported its address yet.</p>
          )}
          <span className="readout text-xs text-bone-faint">
            panel reaches it at {server.panelUrl}
          </span>
        </div>
      </div>
    </section>
  );
}

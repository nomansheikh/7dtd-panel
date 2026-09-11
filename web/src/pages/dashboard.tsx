import { AlertTriangle } from "lucide-react";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Skeleton } from "@/components/ui/skeleton";
import { BloodMoon } from "@/components/blood-moon";
import { useDashboard } from "@/hooks/use-dashboard";
import { formatGameClock, formatUptime } from "@/lib/format";

/** A reading in the stat row. Values share one container, divided by hairlines. */
function Reading({ label, value, hint }: { label: string; value: string; hint?: string }) {
  return (
    <div className="border-border px-5 py-4 sm:border-l sm:first:border-l-0">
      <div className="text-sm text-muted-foreground">{label}</div>
      <div className="mt-1 text-2xl font-semibold readout">{value}</div>
      {hint && <div className="mt-0.5 text-xs text-muted-foreground readout">{hint}</div>}
    </div>
  );
}

export function DashboardPage() {
  const { data, error, isLoading } = useDashboard();

  if (isLoading && !data) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-36 w-full" />
        <Skeleton className="h-24 w-full" />
      </div>
    );
  }

  if (!data) {
    return (
      <Alert variant="destructive">
        <AlertTriangle />
        <AlertTitle>Could not load the dashboard</AlertTitle>
        <AlertDescription>
          {error instanceof Error ? error.message : "The panel did not answer."}
        </AlertDescription>
      </Alert>
    );
  }

  const { world, players, server, uptime, bloodMoon, status, lastError } = data;
  // The panel already extrapolates uptime server-side and freezes it when the
  // server stops answering, so this just renders what it is given.
  const uptimeSeconds = uptime ? uptime.seconds : null;

  return (
    <div className="space-y-6">
      {/* The game server is unreachable but the panel is fine. Say which. */}
      {status === "offline" && (
        <Alert variant="destructive">
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

      <div className="grid gap-6 lg:grid-cols-[1fr_20rem]">
        {/*
          The hero: where the world is in its cycle. Deliberately not wrapped in
          a card, so it reads as the page's subject rather than one tile
          among equals.
        */}
        <section aria-label="World clock">
          <h2 className="text-sm font-medium text-muted-foreground">{world.name}</h2>
          <div className="mt-2 flex flex-wrap items-baseline gap-x-5 gap-y-1">
            <p className="text-6xl font-semibold readout sm:text-7xl">Day {world.day}</p>
            <p className="text-4xl text-muted-foreground readout sm:text-5xl">
              {formatGameClock(world.hour, world.minute)}
            </p>
          </div>
          <dl className="mt-5 flex flex-wrap gap-x-8 gap-y-2 text-sm">
            <div className="flex gap-2">
              <dt className="text-muted-foreground">Mode</dt>
              <dd>{server.gameMode || "unknown"}</dd>
            </div>
            <div className="flex gap-2">
              <dt className="text-muted-foreground">Version</dt>
              <dd className="readout">{server.version || "unknown"}</dd>
            </div>
          </dl>
        </section>

        {bloodMoon ? (
          <BloodMoon
            currentDay={world.day}
            nextDay={bloodMoon.nextDay}
            nextHour={bloodMoon.nextHour}
            active={bloodMoon.active}
          />
        ) : (
          <section aria-label="Blood moon" className="rounded-md border border-border bg-card p-5">
            <h2 className="text-sm font-medium text-muted-foreground">Blood moon</h2>
            <p className="mt-2 text-sm text-muted-foreground">
              Waiting for the first reading from the server.
            </p>
          </section>
        )}
      </div>

      {/* One container with internal dividers, not four floating cards. */}
      <section
        aria-label="Current readings"
        className="grid grid-cols-2 rounded-md border border-border bg-card sm:grid-cols-4"
      >
        <Reading label="Players" value={`${players.online} / ${players.max || "?"}`} />
        <Reading label="Hostiles" value={String(world.hostiles)} />
        <Reading label="Animals" value={String(world.animals)} />
        <Reading
          label="Uptime"
          value={uptimeSeconds === null ? "—" : formatUptime(uptimeSeconds)}
          hint={uptimeSeconds === null ? "not sampled yet" : undefined}
        />
      </section>
    </div>
  );
}

import { cn } from "@/lib/utils";
import { formatAge } from "@/lib/format";
import type { ServerStatus } from "@/lib/api";

const LABELS: Record<ServerStatus, string> = {
  online: "online",
  degraded: "not responding",
  offline: "offline",
  unknown: "connecting",
};

const DOT: Record<ServerStatus, string> = {
  online: "bg-status-online",
  degraded: "bg-status-degraded",
  offline: "bg-status-offline",
  unknown: "bg-status-unknown",
};

interface Props {
  status: ServerStatus;
  ageSeconds: number;
  stale: boolean;
}

/**
 * The panel's honesty indicator. It reports staleness rather than hiding it,
 * because the figures on screen may be a cached reading from before the game
 * server stopped answering.
 */
export function ConnectionStatus({ status, ageSeconds, stale }: Props) {
  return (
    <div className="flex items-center gap-2 text-sm">
      <span
        aria-hidden
        className={cn(
          "size-2 shrink-0 rounded-full",
          DOT[status],
          // Only a live connection pulses. A stopped clock should look stopped.
          status === "online" && "animate-pulse",
        )}
      />
      <span className="text-foreground">{LABELS[status]}</span>
      {stale && status !== "unknown" && (
        <span className="text-muted-foreground readout">showing {formatAge(ageSeconds)}</span>
      )}
    </div>
  );
}

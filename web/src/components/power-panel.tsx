import { useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { PowerProgress } from "@/components/power-progress";
import { useCancelPower, usePower, useStartPower } from "@/hooks/use-power";
import { cn } from "@/lib/utils";

/**
 * Stopping the server, and finding out whether it came back.
 *
 * The panel can stop a server and can never start one — it talks to the server
 * over that server's own API, so once the process is gone there is nothing to
 * talk to. Whether a stop turns into a restart depends on something outside the
 * panel. That is said here rather than hidden behind a button labelled
 * "restart", because an operator who finds out the hard way finds out at five
 * in the morning.
 */

const COUNTDOWNS = [
  { minutes: 0, label: "Right now" },
  { minutes: 1, label: "In 1 minute" },
  { minutes: 5, label: "In 5 minutes" },
  { minutes: 15, label: "In 15 minutes" },
  { minutes: 30, label: "In 30 minutes" },
  { minutes: 60, label: "In an hour" },
];

export function PowerPanel() {
  const { data: status } = usePower();
  const start = useStartPower();
  const cancel = useCancelPower();

  const [minutes, setMinutes] = useState(5);
  const [reason, setReason] = useState("");

  const phase = status?.phase ?? "idle";
  const busy = phase === "countdown" || phase === "saving" || phase === "stopping";
  const watching = phase === "watching";

  const go = (intent: "restart" | "stop") =>
    start.mutate(
      { intent, minutes, reason: reason.trim() },
      {
        onSuccess: () =>
          toast.success(
            minutes === 0
              ? `Server ${intent === "restart" ? "restarting" : "stopping"} now`
              : `Server ${intent === "restart" ? "restarts" : "stops"} in ${minutes} minutes`,
          ),
        onError: (err) => toast.error("Not started", { description: err.message }),
      },
    );

  return (
    <section>
      <div className="region-head">
        <span className="stencil">Power</span>
        {(busy || watching) && (
          <span className={cn("stencil", watching ? "text-ember" : "text-crimson-lit")}>
            {watching ? "watching" : phase}
          </span>
        )}
      </div>

      <div className="space-y-4 px-4 py-4 md:px-6">
        <PowerProgress status={status} onCancel={() => cancel.mutate()} />

        {!busy && !watching && (
          <>
            <div className="flex flex-wrap items-center gap-2">
              <Select value={String(minutes)} onValueChange={(v) => setMinutes(Number(v))}>
                <SelectTrigger
                  className="h-8 w-auto gap-1.5 px-2 text-xs data-[size=default]:h-8"
                  aria-label="How long until it stops"
                >
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {COUNTDOWNS.map((c) => (
                    <SelectItem key={c.minutes} value={String(c.minutes)} className="text-xs">
                      {c.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>

              <Input
                value={reason}
                onChange={(e) => setReason(e.target.value)}
                placeholder="Why, for the players (optional)"
                className="h-8 min-w-48 flex-1 text-xs"
                aria-label="Reason, broadcast to players"
              />
            </div>

            <div className="flex flex-wrap gap-2">
              <Button disabled={start.isPending} onClick={() => go("restart")}>
                Restart
              </Button>
              <Button
                variant="outline"
                disabled={start.isPending}
                onClick={() => go("stop")}
                className="hover:text-crimson-lit"
              >
                Stop
              </Button>
            </div>

            <p className="max-w-prose text-2xs text-bone-faint">
              Players are warned at intervals, the world is saved, then the server is told to stop.
              The panel keeps watching afterwards and says whether it came back.
              <br />
              <span className="text-bone-dim">The panel cannot start a server.</span> A restart only
              happens if something else brings it back, and a Docker restart policy is not always
              enough — it fires when the container's main process exits, so a wrapper script that
              keeps running will hide a stopped game. If a restart does not come back, that is
              usually why.
            </p>
          </>
        )}
      </div>
    </section>
  );
}

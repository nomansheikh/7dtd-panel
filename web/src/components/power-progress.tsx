import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { type PowerStatus } from "@/lib/api";
import { cn } from "@/lib/utils";

/**
 * Where the sequence has got to, and how it ended.
 *
 * The outcome is the reason this exists. Nobody watches a restart at five in
 * the morning, so the panel has to answer "did it come back" without being
 * asked — and say plainly when it did not, rather than showing a spinner
 * forever or quietly calling it a success.
 */
export function PowerProgress({
  status,
  onCancel,
}: {
  status?: PowerStatus;
  onCancel: () => void;
}) {
  const phase = status?.phase ?? "idle";
  const stopAt = status?.stopAt ? new Date(status.stopAt).getTime() : 0;
  const [now, setNow] = useState(() => Date.now());

  // Only tick while there is a countdown to tick.
  useEffect(() => {
    if (phase !== "countdown") return;
    const id = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(id);
  }, [phase]);

  if (phase === "idle") return null;

  if (phase === "countdown") {
    const left = Math.max(0, Math.round((stopAt - now) / 1000));
    return (
      <div className="panel flex flex-wrap items-center gap-x-4 gap-y-2 p-3">
        <div>
          <span className="stencil">
            {status?.intent === "stop" ? "Stopping in" : "Restarting in"}
          </span>
          <p className="figure mt-1 text-3xl text-bone">{clock(left)}</p>
        </div>
        <p className="max-w-prose flex-1 text-2xs text-bone-faint">
          Players are being warned as it counts down. This runs on the panel, so closing this tab
          will not stop it.
        </p>
        <Button variant="outline" size="sm" className="h-7 text-xs" onClick={onCancel}>
          Call it off
        </Button>
      </div>
    );
  }

  if (phase === "saving" || phase === "stopping" || phase === "watching") {
    const said = {
      saving: "Saving the world…",
      stopping: "Telling the server to stop…",
      watching: "Stopped. Watching to see whether it comes back…",
    }[phase];
    return (
      <p className="panel p-3 text-xs text-bone-dim">
        {said}
        {phase === "watching" && (
          <span className="mt-1 block text-2xs text-bone-faint">
            Up to five minutes. A large world takes a while to load.
          </span>
        )}
      </p>
    );
  }

  const tone =
    phase === "back" ? "text-bone" : phase === "cancelled" ? "text-bone-dim" : "text-ember";

  return (
    <p className={cn("panel p-3 text-xs", tone)}>
      {phase === "back" && (
        <>
          It came back after{" "}
          <span className="readout text-bone">{clock(status?.downSeconds ?? 0)}</span>.
        </>
      )}
      {phase === "cancelled" && <>Called off. Nothing was stopped.</>}
      {phase === "gone" && <>{status?.note}</>}
      {phase === "failed" && (
        <span className="text-destructive">That did not work: {status?.problem}</span>
      )}
    </p>
  );
}

/** Seconds as m:ss, because a countdown is read at a glance. */
function clock(seconds: number): string {
  const m = Math.floor(seconds / 60);
  const s = seconds % 60;
  return `${m}:${String(s).padStart(2, "0")}`;
}

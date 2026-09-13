import { RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { StatusDot } from "@/components/connection-status";
import type { ServerStatus } from "@/lib/api";
import { cn } from "@/lib/utils";

/** The two readouts that sit under the layer switches. */

/**
 * Says how current the ground is, and offers to go and look again.
 *
 * The button is here rather than on the map because it is not a view control:
 * it fetches every square on screen afresh, ignoring every cache between here
 * and the game server. That is the right thing after running visitmap, and the
 * wrong thing to do on a timer.
 */
export function MapFreshness({
  status,
  playing,
  refreshing,
  onRefresh,
}: {
  status: ServerStatus;
  playing: boolean;
  refreshing: boolean;
  onRefresh: () => void;
}) {
  const live = status === "online";
  return (
    <div className="region">
      <div className="region-head">
        <span className="stencil flex-1">Ground</span>
        <StatusDot status={status} />
        <span className="text-2xs text-bone-dim">
          {live ? (playing ? "live" : "idle") : "not updating"}
        </span>
      </div>
      <div className="space-y-2 p-4">
        <p className="text-2xs text-bone-faint">
          {!live
            ? "The game server is not answering, so the map is whatever was last drawn."
            : playing
              ? "Somebody is playing, so the map checks for newly drawn ground every 15 seconds."
              : "Nobody is in the world, so nothing is being drawn and the map is not asking."}
        </p>
        <Button
          variant="outline"
          size="sm"
          className="w-full"
          onClick={onRefresh}
          disabled={refreshing}
        >
          <RefreshCw className={cn(refreshing && "animate-spin")} aria-hidden />
          {refreshing ? "Looking" : "Look again now"}
        </Button>
        <p className="text-2xs text-bone-faint">
          Fetches every square on screen again, ignoring what the browser already has. Worth it
          after rendering the map from the console.
        </p>
      </div>
    </div>
  );
}

export function MapScale({ worldSize, maxZoom }: { worldSize: number; maxZoom: number }) {
  return (
    <div className="region">
      <div className="region-head">
        <span className="stencil">This world</span>
      </div>
      <dl className="grid grid-cols-2 gap-x-3 gap-y-1 px-4 py-3 text-sm">
        <dt className="text-bone-dim">Width</dt>
        <dd className="readout text-right">{worldSize.toLocaleString()} blocks</dd>
        <dt className="text-bone-dim">Closest zoom</dt>
        <dd className="readout text-right">1 block per pixel</dd>
      </dl>
      <p className="px-4 pb-3 text-2xs text-bone-faint">
        At the closest zoom the whole world is {worldSize.toLocaleString()} pixels across, which is{" "}
        {((worldSize / 128) ** 2).toLocaleString()} squares. Only the ones on screen are ever
        fetched, and each is kept until the server has had a chance to redraw it.
      </p>
      <p className="px-4 pb-3 text-2xs text-bone-faint">Rendered to zoom {maxZoom}.</p>
    </div>
  );
}

import { Maximize, Minimize } from "lucide-react";
import { compass } from "@/components/map/tooltips";

export interface MapPosition {
  x: number;
  z: number;
  zoom: number;
}

/**
 * Where the middle of the view is, in the coordinates the game uses.
 *
 * Opaque, not translucent. This sits on top of rendered terrain — green
 * forest, pale desert, white snow — so anything see-through is legible over
 * some of the world and not the rest, which is worse than a solid chip
 * because it looks fine wherever you happened to be looking when you built it.
 */
export function MapCrosshair({ position }: { position: MapPosition | null }) {
  if (!position) return null;
  return (
    <div className="pointer-events-none absolute bottom-2 left-2 z-[500] border border-border bg-background px-2 py-1">
      <span className="readout text-2xs text-bone">
        {compass(position.x, position.z)} · zoom {position.zoom}
      </span>
    </div>
  );
}

/**
 * Fills the screen with the map.
 *
 * Takes the layer switches and the readouts with it rather than the map
 * alone: full screen is for looking at more world, not for losing the ability
 * to turn the zombies on.
 */
export function MapFullscreenButton({
  isFullscreen,
  onToggle,
}: {
  isFullscreen: boolean;
  onToggle: () => void;
}) {
  const Icon = isFullscreen ? Minimize : Maximize;
  return (
    <button
      type="button"
      onClick={onToggle}
      aria-label={
        isFullscreen ? "Leave full screen" : "Show the map full screen"
      }
      className="absolute top-2 right-2 z-[500] flex size-7 items-center justify-center border border-border bg-background text-bone-dim transition-colors hover:bg-accent hover:text-bone"
    >
      <Icon className="size-3.5" aria-hidden />
    </button>
  );
}

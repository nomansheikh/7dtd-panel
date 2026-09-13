import type { MapMarker } from "@/lib/api";

/** Player names and claim owners come from the game and land in HTML. */
export function escape(text: string): string {
  const el = document.createElement("span");
  el.textContent = text;
  return el.innerHTML;
}

/*
Three bare numbers mean nothing, and the axes are not a map reader's: x runs
east, z north, y is height. This is also the order teleportplayer takes.
*/
export function gameCoordinates(marker: MapMarker): string {
  const axis = (letter: string, value: number) =>
    `<span class="map-axis">${letter}</span>&nbsp;<span class="readout">${Math.round(value)}</span>`;
  return [axis("x", marker.x), axis("y", marker.y), axis("z", marker.z)].join(
    '<span class="map-axis">&nbsp;·&nbsp;</span>',
  );
}

/* "-8 N" is not a place, so the letter flips with the sign, as the game's own
   map client does. */
export function compass(x: number, z: number): string {
  const bearing = (value: number, positive: string, negative: string) => {
    if (value === 0) return "0";
    return `${Math.abs(Math.round(value))} ${value > 0 ? positive : negative}`;
  };
  return `${bearing(x, "E", "W")} · ${bearing(z, "N", "S")}`;
}

/** Who this is, and where to find them. */
export function playerTooltip(marker: MapMarker): string {
  return `<span class="map-tip-name">${escape(marker.name)}</span><br>${gameCoordinates(marker)}`;
}

/* A lapsed claim is the one worth spotting: still there, still drawn, but no
   longer protecting the base under it. */
export function claimTooltip(marker: MapMarker): string {
  const owner = marker.owner ? escape(marker.owner) : "unknown owner";
  const size = marker.size ?? 0;
  const state = marker.active
    ? `protects ${size}&nbsp;&times;&nbsp;${size} blocks`
    : "lapsed, protecting nothing";
  return (
    `<span class="map-tip-name">${owner}</span><br>` +
    `<span class="map-tip-note">${state}</span><br>` +
    gameCoordinates(marker)
  );
}

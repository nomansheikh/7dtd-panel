import L from "leaflet";
import type { MapMarker } from "@/lib/api";
import type { MapPalette } from "@/components/map/palette";

/** Where a marker sits: Leaflet's lat is the world's x, its lng the world's z. */
export function at(marker: MapMarker): L.LatLng {
  return L.latLng(marker.x, marker.z);
}

export function coordinates(marker: MapMarker): string {
  return `${Math.round(marker.x)} ${Math.round(marker.y)} ${Math.round(marker.z)}`;
}

/** One overlay, seen from the map: it catches up, repaints, and stops. */
export interface MarkerSync {
  sync(markers: MapMarker[]): void;
  clear(): void;
  /** Repaint in a new palette, for when the theme is switched under it. */
  restyle(palette: MapPalette): void;
}

/**
 * Keeps a layer group in step with a list of markers, moving what is already
 * drawn rather than redrawing it.
 *
 * A blood moon puts hundreds of zombies on the map and they are re-read every
 * few seconds. Clearing the group and rebuilding it each time would throw away
 * and recreate every object on every poll, which shows up as a stutter exactly
 * when the map is most worth watching. Instead each marker is found by id and
 * moved; only arrivals are created and only departures are removed.
 */
export class LiveMarkers<T extends L.Layer> implements MarkerSync {
  private readonly drawn = new Map<string, T>();
  private readonly group: L.LayerGroup;
  private readonly create: (marker: MapMarker) => T;
  private readonly move: (layer: T, marker: MapMarker) => void;
  private readonly identify: (marker: MapMarker) => string;
  private readonly paint?: (layer: T, marker: MapMarker, palette: MapPalette) => void;
  private readonly of = new Map<string, MapMarker>();

  constructor(
    group: L.LayerGroup,
    create: (marker: MapMarker) => T,
    move: (layer: T, marker: MapMarker) => void,
    identify: (marker: MapMarker) => string = (m) => String(m.id),
    paint?: (layer: T, marker: MapMarker, palette: MapPalette) => void,
  ) {
    this.group = group;
    this.create = create;
    this.move = move;
    this.identify = identify;
    this.paint = paint;
  }

  restyle(palette: MapPalette) {
    if (!this.paint) return;
    for (const [id, layer] of this.drawn) {
      const marker = this.of.get(id);
      if (marker) this.paint(layer, marker, palette);
    }
  }

  sync(markers: MapMarker[]) {
    const seen = new Set<string>();

    for (const marker of markers) {
      const id = this.identify(marker);
      seen.add(id);

      this.of.set(id, marker);

      const existing = this.drawn.get(id);
      if (existing) {
        this.move(existing, marker);
        continue;
      }
      const created = this.create(marker);
      this.drawn.set(id, created);
      this.group.addLayer(created);
    }

    for (const [id, layer] of this.drawn) {
      if (seen.has(id)) continue;
      this.group.removeLayer(layer);
      this.drawn.delete(id);
      this.of.delete(id);
    }
  }

  clear() {
    this.group.clearLayers();
    this.drawn.clear();
    this.of.clear();
  }
}

/**
 * Entity markers are circles drawn on a canvas rather than DOM elements.
 *
 * Each DOM marker is an element the browser lays out and paints; several
 * hundred of them moving every few seconds is what makes a live map crawl. One
 * canvas draws them all in a single pass.
 */
export function entityMarkers(
  group: L.LayerGroup,
  renderer: L.Canvas,
  palette: MapPalette,
  kind: "hostile" | "animal",
): LiveMarkers<L.CircleMarker> {
  return new LiveMarkers<L.CircleMarker>(
    group,
    (marker) =>
      L.circleMarker(at(marker), {
        renderer,
        radius: 3,
        interactive: false,
        ...palette[kind],
      }),
    (layer, marker) => layer.setLatLng(at(marker)),
    undefined,
    (layer, _marker, next) => layer.setStyle(next[kind]),
  );
}

/**
 * Player markers are DOM elements, because there are never many of them and
 * each carries a name that has to be readable.
 */
export function playerMarkers(group: L.LayerGroup): LiveMarkers<L.Marker> {
  const icon = (name: string) =>
    L.divIcon({
      className: "map-player",
      html: `<span class="map-player-dot"></span><span class="map-player-name">${escape(name)}</span>`,
      iconSize: [0, 0],
    });

  return new LiveMarkers<L.Marker>(
    group,
    (marker) => {
      const drawn = L.marker(at(marker), { icon: icon(marker.name), keyboard: false });
      drawn.bindTooltip(`${marker.name} · ${coordinates(marker)}`, { direction: "top" });
      return drawn;
    },
    (layer, marker) => {
      layer.setLatLng(at(marker));
      layer.setTooltipContent(`${marker.name} · ${coordinates(marker)}`);
    },
  );
}

/**
 * Land claims are drawn as the square they actually protect, at the size the
 * server reports, rather than as a point: the protected area is the thing an
 * operator needs to see.
 */
export function claimMarkers(
  group: L.LayerGroup,
  renderer: L.Canvas,
  palette: MapPalette,
): LiveMarkers<L.Rectangle> {
  const style = (marker: MapMarker, from: MapPalette) =>
    marker.active ? from.claim : from.claimLapsed;

  const square = (marker: MapMarker) => {
    const half = Math.floor((marker.size ?? 1) / 2);
    return L.latLngBounds(
      L.latLng(marker.x - half, marker.z - half),
      L.latLng(marker.x + half, marker.z + half),
    );
  };

  return new LiveMarkers<L.Rectangle>(
    group,
    (marker) => {
      const drawn = L.rectangle(square(marker), { renderer, ...style(marker, palette) });
      drawn.bindTooltip(claimLabel(marker), { direction: "top" });
      return drawn;
    },
    (layer, marker) => layer.setBounds(square(marker)),
    // Claims have no entity id of their own, so their position identifies them.
    (marker) => `${marker.platformId}:${marker.x},${marker.z}`,
    (layer, marker, next) => layer.setStyle(style(marker, next)),
  );
}

function claimLabel(marker: MapMarker): string {
  const owner = marker.owner || "unknown owner";
  const state = marker.active ? "active" : "lapsed — no longer protecting";
  return `${escape(owner)} · ${state} · ${coordinates(marker)}`;
}

/** Player names come from the game and land in HTML, so they are escaped. */
function escape(text: string): string {
  const el = document.createElement("span");
  el.textContent = text;
  return el.innerHTML;
}

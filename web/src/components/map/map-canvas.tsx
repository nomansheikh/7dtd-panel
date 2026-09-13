import { useCallback, useEffect, useRef } from "react";
import L from "leaflet";
import "leaflet/dist/leaflet.css";
import type { MapConfig, MapLayerName, MapMarker } from "@/lib/api";
import { BLANK_TILE, tileLayer, worldBounds, worldCRS } from "@/components/map/projection";
import {
  claimMarkers,
  entityMarkers,
  playerMarkers,
  type MarkerSync,
} from "@/components/map/marker-layers";
import { readMapPalette } from "@/components/map/palette";
import { useTileRefresh } from "@/components/map/use-tile-refresh";
import { useTheme } from "@/hooks/use-theme";

interface MapCanvasProps {
  config: MapConfig;
  tileTemplate: string;
  markers: Partial<Record<MapLayerName, MapMarker[]>>;
  shown: MapLayerName[];
  onPositionChange?: (position: { x: number; z: number; zoom: number } | null) => void;
  /**
   * Whether any ground has actually been drawn.
   *
   * The map's own config cannot be trusted for this. Setting EnableMapRendering
   * on a running server changes what it reports without starting the renderer,
   * so a server can answer "enabled" and still have no tiles at all. Counting
   * what arrives is the only honest signal.
   */
  onTilesSeen?: (any: boolean) => void;
  /** How often to look for newly drawn ground. See useTileRefresh. */
  refreshMs?: number;
  /** Set to a new value to fetch every square in view again, now. */
  refreshNonce?: number;
}

/** One overlay's group and the thing that keeps it in step with the data. */
type Overlay = { group: L.LayerGroup; live: MarkerSync };

/**
 * The map itself.
 *
 * Leaflet is driven imperatively through a ref rather than wrapped in
 * components, because the expensive part of a live map is exactly the part
 * React would fight: markers must be moved in place across a poll, not
 * unmounted and remounted. The instance is built once and then only ever
 * updated.
 */
export function MapCanvas({
  config,
  tileTemplate,
  markers,
  shown,
  onPositionChange,
  onTilesSeen,
  refreshMs = 0,
  refreshNonce = 0,
}: MapCanvasProps) {
  const { theme } = useTheme();
  const holder = useRef<HTMLDivElement | null>(null);
  const map = useRef<L.Map | null>(null);
  const tiles = useRef<L.TileLayer | null>(null);
  const overlays = useRef<Partial<Record<MapLayerName, Overlay>>>({});

  // Builds a tile layer and wires up the "has anything been drawn" count.
  // Used for the first one and for every replacement a refresh lays over it.
  const buildTiles = useCallback(
    (version: number) => {
      const layer = tileLayer(tileTemplate, config, version);
      let drawn = 0;
      // A square the renderer has never drawn also fires tileload: Leaflet
      // puts the blank image in the tile's src when the request fails, and the
      // browser reports that substitute as having loaded. Counting those would
      // make an entirely blank map claim it had ground on it.
      layer.on("tileload", (event: L.TileEvent) => {
        if ((event.tile as HTMLImageElement).src !== BLANK_TILE) drawn += 1;
      });
      // Fires once a whole screenful has settled, so this is asked after the
      // viewport has had its chance rather than after the first tile.
      layer.on("load", () => onTilesSeen?.(drawn > 0));
      return layer;
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [tileTemplate, config.tileSize, config.maxZoom],
  );

  // Built once. Rebuilding it on a prop change would reset the view the
  // operator had panned to.
  useEffect(() => {
    if (!holder.current || map.current) return;

    const instance = L.map(holder.current, {
      crs: worldCRS(config.maxZoom),
      // One canvas for every circle and rectangle, rather than an element each.
      preferCanvas: true,
      center: [0, 0],
      zoom: Math.max(0, config.maxZoom - 3),
      minZoom: 0,
      maxZoom: config.maxZoom + 1,
      maxBounds: worldBounds(config.worldSize),
      maxBoundsViscosity: 0.8,
      zoomControl: false,
      attributionControl: false,
      // Tiles appear at once rather than fading up from nothing.
      //
      // The fade is why refreshing blinked: a replacement layer's tiles start
      // transparent, and the event that says they have loaded fires before the
      // fade has finished, so the old layer was being taken away while the new
      // one was still invisible. Measured at a 100% dip in what was on screen.
      fadeAnimation: false,
    });

    const layer = buildTiles(0);
    tiles.current = layer;
    layer.addTo(instance);
    L.control.zoom({ position: "bottomright" }).addTo(instance);

    const renderer = L.canvas({ padding: 0.3 });
    const palette = readMapPalette();
    const groups: Record<MapLayerName, Overlay> = {
      claims: overlay(instance, (group) => claimMarkers(group, renderer, palette)),
      animals: overlay(instance, (group) => entityMarkers(group, renderer, palette, "animal")),
      hostiles: overlay(instance, (group) => entityMarkers(group, renderer, palette, "hostile")),
      players: overlay(instance, (group) => playerMarkers(group)),
    };
    overlays.current = groups;

    const report = () => {
      const centre = instance.getCenter();
      onPositionChange?.({
        x: Math.round(centre.lat),
        z: Math.round(centre.lng),
        zoom: instance.getZoom(),
      });
    };
    instance.on("moveend zoomend", report);
    report();

    map.current = instance;
    return () => {
      instance.remove();
      map.current = null;
      tiles.current = null;
      overlays.current = {};
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Add and remove whole layers as they are switched on and off. A layer that
  // is off is also not fetched, so its markers stop arriving entirely.
  useEffect(() => {
    const instance = map.current;
    if (!instance) return;

    for (const [name, overlay] of Object.entries(overlays.current) as [MapLayerName, Overlay][]) {
      const wanted = shown.includes(name);
      if (wanted && !instance.hasLayer(overlay.group)) {
        instance.addLayer(overlay.group);
      } else if (!wanted && instance.hasLayer(overlay.group)) {
        instance.removeLayer(overlay.group);
        overlay.live.clear();
      }
    }
  }, [shown]);

  // Repaint when the theme changes. The markers are drawn on a canvas, so
  // nothing about them follows a stylesheet: without this they keep the
  // previous theme's colours until the page is reloaded.
  useEffect(() => {
    const palette = readMapPalette();
    for (const overlay of Object.values(overlays.current)) {
      overlay?.live.restyle(palette);
    }
  }, [theme]);

  useTileRefresh({ map, tiles, build: buildTiles, everyMs: refreshMs, nonce: refreshNonce });

  // Move what is drawn to where it now is.
  useEffect(() => {
    for (const [name, overlay] of Object.entries(overlays.current) as [MapLayerName, Overlay][]) {
      if (!shown.includes(name)) continue;
      overlay.live.sync(markers[name] ?? []);
    }
  }, [markers, shown]);

  return <div ref={holder} className="h-full w-full bg-ash-950" />;
}

function overlay(instance: L.Map, build: (group: L.LayerGroup) => MarkerSync): Overlay {
  const group = L.layerGroup();
  const live = build(group);
  instance.addLayer(group);
  return { group, live };
}

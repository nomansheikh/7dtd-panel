import { useEffect, useRef, type RefObject } from "react";
import L from "leaflet";
import type { MapLayerName, MapMarker } from "@/lib/api";
import {
  claimMarkers,
  entityMarkers,
  playerMarkers,
  type MarkerSync,
} from "@/components/map/marker-layers";
import { readMapPalette } from "@/components/map/palette";

/** One overlay's group and the thing that keeps it in step with the data. */
export interface Overlay {
  group: L.LayerGroup;
  live: MarkerSync;
}

/**
 * Builds the four overlays and keeps them in step with what arrives.
 *
 * Every circle and rectangle shares one canvas rather than being an element
 * each: a blood moon puts hundreds of zombies on the map, moving every few
 * seconds, and that many DOM nodes is what makes a live map crawl.
 */
export function useMarkerOverlays(
  map: RefObject<L.Map | null>,
  markers: Partial<Record<MapLayerName, MapMarker[]>>,
  shown: MapLayerName[],
  theme: string,
) {
  const overlays = useRef<Partial<Record<MapLayerName, Overlay>>>({});

  const attach = (instance: L.Map) => {
    const renderer = L.canvas({ padding: 0.3 });
    const palette = readMapPalette();
    overlays.current = {
      claims: build(instance, (group) => claimMarkers(group, renderer, palette)),
      animals: build(instance, (group) => entityMarkers(group, renderer, palette, "animal")),
      hostiles: build(instance, (group) => entityMarkers(group, renderer, palette, "hostile")),
      players: build(instance, (group) => playerMarkers(group)),
    };
  };

  const detach = () => {
    overlays.current = {};
  };

  // Add and remove whole layers as they are switched on and off. A layer that
  // is off is also not fetched, so its markers stop arriving entirely.
  useEffect(() => {
    const instance = map.current;
    if (!instance) return;

    for (const [name, overlay] of entries(overlays.current)) {
      const wanted = shown.includes(name);
      if (wanted && !instance.hasLayer(overlay.group)) {
        instance.addLayer(overlay.group);
      } else if (!wanted && instance.hasLayer(overlay.group)) {
        instance.removeLayer(overlay.group);
        overlay.live.clear();
      }
    }
  }, [map, shown]);

  // Repaint when the theme changes. The markers are drawn on a canvas, so
  // nothing about them follows a stylesheet: without this they keep the
  // previous theme's colours until the page is reloaded.
  useEffect(() => {
    const palette = readMapPalette();
    for (const [, overlay] of entries(overlays.current)) {
      overlay.live.restyle(palette);
    }
  }, [theme]);

  // Move what is drawn to where it now is.
  useEffect(() => {
    for (const [name, overlay] of entries(overlays.current)) {
      if (!shown.includes(name)) continue;
      overlay.live.sync(markers[name] ?? []);
    }
  }, [markers, shown]);

  return { attach, detach };
}

function entries(of: Partial<Record<MapLayerName, Overlay>>): [MapLayerName, Overlay][] {
  return Object.entries(of) as [MapLayerName, Overlay][];
}

function build(instance: L.Map, make: (group: L.LayerGroup) => MarkerSync): Overlay {
  const group = L.layerGroup();
  const live = make(group);
  instance.addLayer(group);
  return { group, live };
}

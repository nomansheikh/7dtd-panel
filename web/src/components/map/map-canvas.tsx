import { useCallback, useEffect, useRef } from "react";
import L from "leaflet";
import "leaflet/dist/leaflet.css";
import type { MapConfig, MapLayerName, MapMarker } from "@/lib/api";
import { BLANK_TILE, tileLayer, worldBounds, worldCRS } from "@/components/map/projection";
import { useMarkerOverlays } from "@/components/map/use-marker-overlays";
import { useTileRefresh } from "@/components/map/use-tile-refresh";
import { useTheme } from "@/hooks/use-theme";

interface MapCanvasProps {
  config: MapConfig;
  tileTemplate: string;
  markers: Partial<Record<MapLayerName, MapMarker[]>>;
  shown: MapLayerName[];
  onPositionChange?: (position: { x: number; z: number; zoom: number } | null) => void;
  /* The map's own config cannot be trusted for this: a server can report
     "enabled" and have no tiles at all. Counting what arrives is the only
     honest signal. */
  onTilesSeen?: (any: boolean) => void;
  /** How often to look for newly drawn ground. See useTileRefresh. */
  refreshMs?: number;
  /** Set to a new value to fetch every square in view again, now. */
  refreshNonce?: number;
  /** Told while a refresh is in flight, so a control can show it working. */
  onRefreshingChange?: (busy: boolean) => void;
  /** Somewhere to move the view to. `at` changing is what triggers the move. */
  focus?: { x: number; z: number; at: number } | null;
  onContextMenu?: (at: { x: number; z: number }) => void;
}

/*
Leaflet is driven imperatively rather than wrapped in components, because the
expensive part of a live map is the part React would fight: markers must be
moved in place across a poll, not unmounted and remounted.
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
  onRefreshingChange,
  focus = null,
  onContextMenu,
}: MapCanvasProps) {
  const { theme } = useTheme();
  const holder = useRef<HTMLDivElement | null>(null);
  const map = useRef<L.Map | null>(null);
  const tiles = useRef<L.TileLayer | null>(null);
  const overlays = useMarkerOverlays(map, markers, shown, theme);

  /* Used for the first layer and every replacement a refresh lays over it. */
  const buildTiles = useCallback(
    (version: number) => {
      const layer = tileLayer(tileTemplate, config, version);
      let drawn = 0;
      /* An undrawn square fires tileload too: Leaflet puts the blank image in
         the tile's src on failure and the browser reports that as loaded. */
      layer.on("tileload", (event: L.TileEvent) => {
        if ((event.tile as HTMLImageElement).src !== BLANK_TILE) drawn += 1;
      });
      layer.on("load", () => onTilesSeen?.(drawn > 0));
      return layer;
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [tileTemplate, config.tileSize, config.maxZoom],
  );

  /* Built once; rebuilding on a prop change would reset the operator's view. */
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
      /* The fade is why refreshing blinked: a replacement layer's tiles start
         transparent and "loaded" fires before the fade ends, so the old layer
         went while the new one was still invisible. Measured at a 100% dip. */
      fadeAnimation: false,
    });

    const layer = buildTiles(0);
    tiles.current = layer;
    layer.addTo(instance);
    L.control.zoom({ position: "bottomright" }).addTo(instance);

    overlays.attach(instance);

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

    /*
    Leaflet calls preventDefault on every contextmenu, and Radix skips its own
    handler when an event arrives already defaultPrevented, so a menu wrapping
    the map would never open. Take the event away from Leaflet and read the
    position off the map directly.
    */
    const container = instance.getContainer();
    const leafletContextMenu = (instance as unknown as { _handleDOMEvent: EventListener })
      ._handleDOMEvent;
    L.DomEvent.off(container, "contextmenu", leafletContextMenu, instance);

    const rightClick = (event: MouseEvent) => {
      const point = instance.mouseEventToLatLng(event);
      onContextMenu?.({ x: point.lat, z: point.lng });
    };
    container.addEventListener("contextmenu", rightClick);

    /* Leaflet caches the container size, so going full screen would otherwise
       leave it drawing into the old rectangle. */
    const resized = new ResizeObserver(() => instance.invalidateSize({ animate: false }));
    resized.observe(holder.current);

    map.current = instance;
    return () => {
      resized.disconnect();
      container.removeEventListener("contextmenu", rightClick);
      instance.remove();
      map.current = null;
      tiles.current = null;
      overlays.detach();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const buildPatch = useCallback(
    (version: number, bounds: L.LatLngBounds) => tileLayer(tileTemplate, config, version, bounds),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [tileTemplate, config.tileSize, config.maxZoom],
  );

  useTileRefresh({
    map,
    tiles,
    players: (markers.players ?? []).map((p) => ({ x: p.x, z: p.z })),
    build: buildTiles,
    buildPatch,
    everyMs: refreshMs,
    nonce: refreshNonce,
    onBusyChange: onRefreshingChange,
  });

  useEffect(() => {
    if (!focus) return;
    const instance = map.current;
    if (!instance) return;
    /* Close enough to see a base, not so close the surroundings are lost. */
    const near = Math.max(instance.getZoom(), config.maxZoom - 1);
    instance.setView([focus.x, focus.z], near, { animate: true });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [focus?.at]);

  return <div ref={holder} className="h-full w-full bg-background" />;
}

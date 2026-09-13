import { useEffect, useRef, type RefObject } from "react";
import L from "leaflet";

/** How long to wait for a replacement layer before dropping the old one anyway. */
const GIVE_UP_AFTER = 20_000;

/* A refresh of cached squares finishes inside a frame, and feedback that
   brief reads as nothing having happened. */
const MIN_VISIBLE = 400;

/* How far around a player the renderer can have drawn since the last look. */
const DREW_WITHIN = 256;

interface TileRefresh {
  map: RefObject<L.Map | null>;
  tiles: RefObject<L.TileLayer | null>;
  /* Where the online players are. Ground is only ever drawn around them. */
  players: { x: number; z: number }[];
  /* Builds a patch layer covering just one area. */
  buildPatch: (version: number, bounds: L.LatLngBounds) => L.TileLayer;
  /** Builds a replacement layer. version is filled into {v} in the tile URL. */
  build: (version: number) => L.TileLayer;
  /** How often to look for newly drawn ground. Zero switches it off. */
  everyMs: number;
  /** Changes when somebody asks for a refresh that ignores every cache. */
  nonce: number;
  /* Told only about an explicit refresh. The interval is meant to go
     unnoticed; a control blinking every fifteen seconds on its own is noise. */
  onBusyChange?: (busy: boolean) => void;
}

/*
Leaflet asks for a tile once, when it enters the view, and never again, so
ground rendered while somebody is watching would never appear.

redraw() is the obvious fix and is wrong: it pulls every tile out of the page
first, blinking the whole map. A second layer is laid over the first instead
and the first removed once the replacement has loaded.

The interval keeps the same URLs so the browser revalidates and is told 304;
the explicit refresh changes the URL so every square is fetched again.
*/
export function useTileRefresh({
  map,
  tiles,
  players,
  build,
  buildPatch,
  everyMs,
  nonce,
  onBusyChange,
}: TileRefresh) {
  /* Read through a ref so the interval always sees the current positions
     without being torn down and rebuilt every time somebody takes a step. */
  const where = useRef(players);
  where.current = players;
  const patch = useRef<L.TileLayer | null>(null);
  /* In a ref so the interval calls the current one without being torn down
     and rebuilt whenever a dependency changes. */
  const swap = useRef<(version: number, announce: boolean) => void>(() => {});
  /* Refreshes can overlap, so a count rather than a flag. */
  const running = useRef(0);
  const report = useRef<((busy: boolean) => void) | undefined>(undefined);
  report.current = onBusyChange;

  swap.current = (version: number, announce: boolean) => {
    const instance = map.current;
    const previous = tiles.current;
    if (!instance || !previous) return;

    const replacement = build(version);
    const startedAt = Date.now();
    if (announce) {
      running.current += 1;
      report.current?.(true);
    }

    let retired = false;
    const retire = () => {
      if (retired) return;
      retired = true;
      instance.removeLayer(previous);

      if (!announce) return;
      const linger = Math.max(0, MIN_VISIBLE - (Date.now() - startedAt));
      window.setTimeout(() => {
        running.current -= 1;
        if (running.current === 0) report.current?.(false);
      }, linger);
    };

    replacement.on("load", retire);
    /* A server that stops answering mid-refresh would otherwise leave the old
       layer for ever, and they would pile up. */
    window.setTimeout(retire, GIVE_UP_AFTER);

    replacement.addTo(instance);
    tiles.current = replacement;
  };

  /*
  Only the ground around a player can have changed, so only that is fetched
  again. Rebuilding the whole viewport on a timer created and destroyed every
  tile on screen — 160 of them full screen, more on a wide monitor — every
  fifteen seconds, for an area where nothing could possibly have been drawn.

  The patch is laid over the base layer and the previous patch retired once it
  has loaded, so nothing blinks.
  */
  useEffect(() => {
    if (everyMs <= 0) return;
    const timer = window.setInterval(() => {
      /* Nothing while the tab is in the background: a map on a second monitor
         should not keep a game server busy all night. */
      if (document.visibilityState !== "visible") return;
      const instance = map.current;
      const here = where.current;
      if (!instance || here.length === 0) return;

      const bounds = L.latLngBounds(
        here.flatMap((p) => [
          L.latLng(p.x - DREW_WITHIN, p.z - DREW_WITHIN),
          L.latLng(p.x + DREW_WITHIN, p.z + DREW_WITHIN),
        ]),
      );

      const previous = patch.current;
      const replacement = buildPatch(Date.now(), bounds);
      const retire = () => previous && instance.removeLayer(previous);
      replacement.on("load", retire);
      window.setTimeout(retire, GIVE_UP_AFTER);
      replacement.addTo(instance);
      patch.current = replacement;
    }, everyMs);
    return () => window.clearInterval(timer);
  }, [everyMs, map, buildPatch]);

  useEffect(() => {
    if (nonce <= 0) return;
    swap.current(nonce, true);
  }, [nonce]);
}

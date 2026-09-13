import { useEffect, useRef, type RefObject } from "react";
import type L from "leaflet";

/** How long to wait for a replacement layer before dropping the old one anyway. */
const GIVE_UP_AFTER = 20_000;

/* A refresh of cached squares finishes inside a frame, and feedback that
   brief reads as nothing having happened. */
const MIN_VISIBLE = 400;

interface TileRefresh {
  map: RefObject<L.Map | null>;
  tiles: RefObject<L.TileLayer | null>;
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
export function useTileRefresh({ map, tiles, build, everyMs, nonce, onBusyChange }: TileRefresh) {
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

  useEffect(() => {
    if (everyMs <= 0) return;
    const timer = window.setInterval(() => {
      /* Nothing while the tab is in the background: a map on a second monitor
         should not keep a game server busy all night. */
      if (document.visibilityState !== "visible") return;
      const version = (tiles.current?.options as { v?: number } | undefined)?.v ?? 0;
      swap.current(version, false);
    }, everyMs);
    return () => window.clearInterval(timer);
  }, [everyMs, tiles]);

  useEffect(() => {
    if (nonce <= 0) return;
    swap.current(nonce, true);
  }, [nonce]);
}

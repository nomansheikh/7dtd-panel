import { useEffect, useRef, type RefObject } from "react";
import type L from "leaflet";

/** How long to wait for a replacement layer before dropping the old one anyway. */
const GIVE_UP_AFTER = 20_000;

interface TileRefresh {
  map: RefObject<L.Map | null>;
  tiles: RefObject<L.TileLayer | null>;
  /** Builds a replacement layer. version is filled into {v} in the tile URL. */
  build: (version: number) => L.TileLayer;
  /** How often to look for newly drawn ground. Zero switches it off. */
  everyMs: number;
  /** Changes when somebody asks for a refresh that ignores every cache. */
  nonce: number;
}

/**
 * Goes back and looks for ground drawn since the map was opened.
 *
 * Leaflet asks for a tile once, when it enters the view, and never again. On an
 * ordinary map that is right — nobody is redrawing the world while you look at
 * it. Here they are: a player walking into country nobody has visited makes the
 * server render it, and without this the map somebody is watching stays exactly
 * as it was when they opened it.
 *
 * The obvious way to do this is redraw(), and it is wrong: redraw() pulls every
 * tile out of the page and puts them back, so the whole map blinks each time.
 * Instead a second layer is laid over the first and the first is only removed
 * once the replacement has finished loading, which is invisible.
 *
 * Two kinds of refresh, because they cost very different amounts. The interval
 * keeps the same URLs, so the browser revalidates and is told 304 for every
 * square that has not changed. The explicit one changes the URL, so every
 * square in view is fetched again whatever any cache in between believes.
 *
 * Nothing happens while the tab is in the background: a map left open on a
 * second monitor should not keep a game server busy all night.
 */
export function useTileRefresh({ map, tiles, build, everyMs, nonce }: TileRefresh) {
  // Held in a ref so the interval always calls the current one without being
  // torn down and rebuilt every time a dependency changes.
  const swap = useRef<(version: number) => void>(() => {});

  swap.current = (version: number) => {
    const instance = map.current;
    const previous = tiles.current;
    if (!instance || !previous) return;

    const replacement = build(version);
    let retired = false;
    const retire = () => {
      if (retired) return;
      retired = true;
      instance.removeLayer(previous);
    };

    // Only once the new tiles are up. A layer whose squares are all cached
    // still fires this, on the next frame.
    replacement.on("load", retire);
    // If it never finishes — a server that stops answering mid-refresh — the
    // old layer would otherwise stay for ever and they would pile up.
    window.setTimeout(retire, GIVE_UP_AFTER);

    replacement.addTo(instance);
    tiles.current = replacement;
  };

  useEffect(() => {
    if (everyMs <= 0) return;
    const timer = window.setInterval(() => {
      if (document.visibilityState !== "visible") return;
      // Same version, so the URLs are unchanged and the browser revalidates.
      const version = (tiles.current?.options as { v?: number } | undefined)?.v ?? 0;
      swap.current(version);
    }, everyMs);
    return () => window.clearInterval(timer);
  }, [everyMs, tiles]);

  useEffect(() => {
    if (nonce <= 0) return;
    swap.current(nonce);
  }, [nonce]);
}

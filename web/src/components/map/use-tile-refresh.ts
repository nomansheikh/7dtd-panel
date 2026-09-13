import { useEffect, type RefObject } from "react";
import type L from "leaflet";

/**
 * Goes back and looks for ground drawn since the map was opened.
 *
 * Leaflet asks for a tile once, when it enters the view, and never again. On an
 * ordinary map that is right — nobody is redrawing the world while you look at
 * it. Here they are: a player walking into country nobody has visited makes the
 * server render it, and without this the map somebody is watching stays exactly
 * as it was when they opened it.
 *
 * Two refreshes, because they cost very different amounts:
 *
 *   - the interval reuses the same URLs, so the browser revalidates and is told
 *     304 for every square that has not changed;
 *   - the explicit one changes the URL, so every square in view is fetched
 *     again whatever any cache in between believes.
 *
 * Nothing happens while the tab is in the background. A map left open on a
 * second monitor should not keep a game server busy all night.
 */
export function useTileRefresh(
  tiles: RefObject<L.TileLayer | null>,
  everyMs: number,
  nonce: number,
) {
  useEffect(() => {
    if (everyMs <= 0) return;
    const timer = window.setInterval(() => {
      if (document.visibilityState !== "visible") return;
      tiles.current?.redraw();
    }, everyMs);
    return () => window.clearInterval(timer);
  }, [tiles, everyMs]);

  useEffect(() => {
    if (nonce <= 0) return;
    const layer = tiles.current;
    if (!layer) return;
    // Filled into {v} in the tile URL template, which is what makes the
    // browser treat these as squares it has never seen.
    (layer.options as { v?: number }).v = nonce;
    layer.redraw();
  }, [tiles, nonce]);
}

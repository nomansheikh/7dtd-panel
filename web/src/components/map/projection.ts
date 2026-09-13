import L from "leaflet";
import type { MapConfig } from "@/lib/api";

/**
 * The coordinate system the game's renderer draws in.
 *
 * Not a guess: this is the same projection the server's own map client uses,
 * read from both the dashboard it ships and the copy of the map viewer in
 * Mods/Allocs_WebAndMapRendering. Getting any part of it wrong puts every
 * marker in the wrong place, which is the kind of bug nobody notices until
 * they go looking for a base.
 *
 * Dividing by 2^maxZoom before scaling by 2^z means that at the deepest zoom
 * one world block is exactly one pixel, which is how the renderer writes the
 * tiles. Leaflet's lat is the world's x (east) and its lng is the world's z
 * (north), so a marker is placed as L.latLng(x, z).
 */
export function worldCRS(maxZoom: number): L.CRS {
  const scale = 2 ** maxZoom;
  return L.extend({}, L.CRS.Simple, {
    projection: {
      project: (latlng: L.LatLng) => new L.Point(latlng.lat / scale, latlng.lng / scale),
      unproject: (point: L.Point) => new L.LatLng(point.x * scale, point.y * scale),
    },
    // The y axis runs the opposite way to Leaflet's default.
    transformation: new L.Transformation(1, 0, -1, 0),
    scale: (zoom: number) => 2 ** zoom,
    zoom: (scale: number) => Math.log(scale) / Math.LN2,
  });
}

/** The square the world occupies, so panning cannot wander off into nothing. */
export function worldBounds(worldSize: number): L.LatLngBounds {
  const half = worldSize / 2;
  return L.latLngBounds(L.latLng(-half, -half), L.latLng(half, half));
}

/**
 * A transparent pixel, used for squares the renderer has never drawn.
 *
 * A data URI rather than a file so that a blank square costs no request at
 * all. Most of a world has never been explored, so on a fresh map this is what
 * nearly every tile resolves to.
 */
export const BLANK_TILE =
  "data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7";

/**
 * The tile layer for one server's rendered map.
 *
 * maxNativeZoom stops Leaflet asking for tiles deeper than the renderer
 * produces: past it, it upscales a tile it already has instead of making a
 * request that can only 404. minNativeZoom does the same at the shallow end.
 * Without both, zooming out on a large world would ask for thousands of tiles
 * that have never existed.
 */
export function tileLayer(template: string, config: MapConfig, version = 0): L.TileLayer {
  // Leaflet fills any extra option into a matching {placeholder} in the
  // template, which its own types do not describe.
  const options = {
    // Filled into {v}. Only a refresh that must ignore caches changes it.
    v: version,
    tileSize: config.tileSize,
    minZoom: 0,
    // One level of over-zoom, upscaled from the deepest real tiles, which is
    // what the game's own viewer allows.
    maxZoom: config.maxZoom + 1,
    minNativeZoom: 0,
    maxNativeZoom: config.maxZoom,
    // A square the renderer has never drawn is blank, not broken.
    errorTileUrl: BLANK_TILE,
    // Nothing is requested mid-zoom: the frames between two zoom levels would
    // otherwise each ask for a screenful of tiles that is thrown away.
    updateWhenZooming: false,
    // Two rings of tiles beyond the viewport, so a small pan draws instantly
    // from what is already loaded.
    keepBuffer: 2,
    className: "map-tiles",
  } as L.TileLayerOptions;

  const layer = L.tileLayer(template, options);

  // The browser counts tile rows downward from the top; the game counts them
  // upward from the bottom. The panel's own API does the flip, so the URL in
  // the network tab matches the tile file on the game server.
  return layer;
}

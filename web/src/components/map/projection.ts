import L from "leaflet";
import type { MapConfig } from "@/lib/api";

/*
The coordinate system the game's renderer draws in, read from two independent
copies of the game's own map client and confirmed against a live server.
Dividing by 2^maxZoom before scaling by 2^z is what makes one world block one
pixel at the deepest zoom, which is how the tiles are written. Leaflet's lat is
the world's x and its lng the world's z.
*/
export function worldCRS(maxZoom: number): L.CRS {
  const scale = 2 ** maxZoom;
  return L.extend({}, L.CRS.Simple, {
    projection: {
      project: (latlng: L.LatLng) => new L.Point(latlng.lat / scale, latlng.lng / scale),
      unproject: (point: L.Point) => new L.LatLng(point.x * scale, point.y * scale),
    },
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

/** A transparent pixel for undrawn squares. A data URI, so it costs no request. */
export const BLANK_TILE =
  "data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7";

export function tileLayer(template: string, config: MapConfig, version = 0): L.TileLayer {
  /* Leaflet fills any extra option into a matching {placeholder} in the
     template, which its own types do not describe. */
  const options = {
    v: version,
    tileSize: config.tileSize,
    minZoom: 0,
    maxZoom: config.maxZoom + 1,
    /* Past maxNativeZoom Leaflet upscales a tile it has instead of asking for
       one that cannot exist. Without both bounds, zooming out on a large world
       requests thousands of tiles that were never rendered. */
    minNativeZoom: 0,
    maxNativeZoom: config.maxZoom,
    errorTileUrl: BLANK_TILE,
    /* Nothing is requested mid-zoom; the frames between two levels would each
       ask for a screenful that is thrown away. */
    updateWhenZooming: false,
    keepBuffer: 2,
    className: "map-tiles",
  } as L.TileLayerOptions;

  return L.tileLayer(template, options);
}

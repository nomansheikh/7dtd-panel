import type L from "leaflet";

/**
 * The colours the map draws its markers in.
 *
 * Read from the panel's own tokens at runtime rather than written here,
 * because a canvas does not read stylesheets: Leaflet paints circles and
 * rectangles straight onto a 2D context, so a CSS class on them does nothing
 * and every marker comes out in Leaflet's default blue. Resolving the tokens
 * keeps index.css the single place a colour is defined, and keeps the map in
 * step when the theme is switched.
 */
export interface MapPalette {
  hostile: L.PathOptions;
  animal: L.PathOptions;
  claim: L.PathOptions;
  claimLapsed: L.PathOptions;
}

function token(name: string): string {
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim();
}

export function readMapPalette(): MapPalette {
  const ember = token("--ember");
  const crimson = token("--crimson");
  const crimsonLit = token("--crimson-lit");

  return {
    hostile: { color: crimsonLit, fillColor: crimson, fillOpacity: 0.85, weight: 1 },
    animal: { color: ember, fillColor: ember, fillOpacity: 0.6, weight: 1 },
    claim: { color: ember, fillColor: ember, fillOpacity: 0.12, weight: 1 },
    // A lapsed claim is protecting nothing, which is worth seeing. Dashed as
    // well as crimson, so the difference is never colour alone.
    claimLapsed: {
      color: crimsonLit,
      fillColor: crimson,
      fillOpacity: 0.05,
      weight: 1,
      dashArray: "3 3",
    },
  };
}

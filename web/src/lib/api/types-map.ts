/** The rendered map's dimensions, as the game's renderer decided them. */
export interface MapConfig {
  /* Must be true in the config the server launched with. Setting it at
     runtime changes what the server reports without starting the renderer. */
  enabled: boolean;
  tileSize: number;
  /** The deepest level rendered. At it, one block is one pixel. */
  maxZoom: number;
  /** The world's width in blocks. Worlds are square. */
  worldSize: number;
}

export type MapLayerName = "players" | "hostiles" | "animals" | "claims";

/** One thing on the map, in the game's axes: x east, z north, y height. */
export interface MapMarker {
  id: number;
  name: string;
  x: number;
  y: number;
  z: number;
  owner?: string;
  platformId?: string;
  /** A land claim's protected width in blocks. Absent for points. */
  size?: number;
  /** Whether a land claim is still keeping its owner's base safe. */
  active?: boolean;
}

export interface MapMarkers {
  layers: Partial<Record<MapLayerName, MapMarker[]>>;
  /** Per layer, why it could not be read; the others still draw. */
  problems: Partial<Record<MapLayerName, string>>;
}

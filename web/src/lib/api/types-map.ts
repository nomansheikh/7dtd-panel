/** The rendered map's dimensions, as the game's renderer decided them. */
export interface MapConfig {
  /**
   * Whether the renderer is running. It must be switched on in the server's
   * own serverconfig.xml before the server starts: no console command and no
   * preference change will start it once the server is up.
   */
  enabled: boolean;
  /** Width and height of one tile image, in pixels and in world blocks. */
  tileSize: number;
  /** The deepest level the renderer produces. At it, one block is one pixel. */
  maxZoom: number;
  /** The world's width in blocks. Worlds are square. */
  worldSize: number;
}

/** The overlays the map can draw. */
export type MapLayerName = "players" | "hostiles" | "animals" | "claims";

/**
 * One thing on the map, in the game's own axes: x runs east, z runs north,
 * and y is height.
 */
export interface MapMarker {
  id: number;
  name: string;
  x: number;
  y: number;
  z: number;
  owner?: string;
  platformId?: string;
  /** A land claim's protected width in blocks. Absent for anything drawn as a point. */
  size?: number;
  /** Whether a land claim is still keeping its owner's base safe. */
  active?: boolean;
}

export interface MapMarkers {
  layers: Partial<Record<MapLayerName, MapMarker[]>>;
  /**
   * Per layer, why it could not be read. A layer the game server refused
   * should not blank out the ones it answered.
   */
  problems: Partial<Record<MapLayerName, string>>;
}

import { request, forServer } from "@/lib/api/request";
import type { MapConfig, MapMarkers, MapLayerName } from "@/lib/api/types-map";

/**
 * The rendered world.
 *
 * Tiles are deliberately absent from this client: they are fetched by the map
 * itself as plain <img> elements so the browser's own cache, its connection
 * pooling and its lazy loading all apply. Routing a few hundred images per pan
 * through fetch() would throw all three away.
 */
export const gameMapApi = {
  mapConfig: (serverId: string) => request<MapConfig>(forServer(serverId, "/map/config")),

  mapMarkers: (serverId: string, layers: MapLayerName[]) =>
    request<MapMarkers>(forServer(serverId, `/map/markers?layers=${layers.join(",")}`)),

  /**
   * The URL template Leaflet fills in per tile.
   *
   * The {v} is a cache buster the map only changes when somebody asks for a
   * hard refresh. Leaving it out of the ordinary refresh is deliberate: a
   * stable URL lets the browser revalidate and be told 304, while a changing
   * one makes it refetch every square in view.
   */
  mapTileTemplate: (serverId: string) => forServer(serverId, "/map/tiles/{z}/{x}/{y}?v={v}"),
};

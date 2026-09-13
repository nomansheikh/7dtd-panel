import { request, forServer } from "@/lib/api/request";
import type { MapConfig, MapMarkers, MapLayerName } from "@/lib/api/types-map";

/*
Tiles are deliberately absent from this client. They are fetched as plain
<img> elements so the browser's own cache, connection pooling and lazy loading
all apply; routing a few hundred images per pan through fetch() throws all
three away.
*/
export const gameMapApi = {
  mapConfig: (serverId: string) => request<MapConfig>(forServer(serverId, "/map/config")),

  mapMarkers: (serverId: string, layers: MapLayerName[]) =>
    request<MapMarkers>(forServer(serverId, `/map/markers?layers=${layers.join(",")}`)),

  /* {v} is a cache buster only a hard refresh changes; a stable URL lets the
     browser revalidate instead of refetching every square. */
  mapTileTemplate: (serverId: string) => forServer(serverId, "/map/tiles/{z}/{x}/{y}?v={v}"),
};

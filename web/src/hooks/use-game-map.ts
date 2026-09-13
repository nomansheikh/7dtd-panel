import { useQuery } from "@tanstack/react-query";
import { api, type MapLayerName } from "@/lib/api";
import { useServerId } from "@/hooks/use-servers";

/**
 * The map's dimensions.
 *
 * Read once and held: the renderer's tile size, zoom depth and world extent are
 * fixed when the server starts and cannot change while it is up.
 */
export function useMapConfig() {
  const serverId = useServerId();
  return useQuery({
    queryKey: ["map-config", serverId],
    queryFn: () => api.mapConfig(serverId),
    enabled: serverId !== "",
    staleTime: Infinity,
  });
}

/** The layers that move, and so are worth asking about often. */
export const MOVING_LAYERS: MapLayerName[] = ["players", "hostiles", "animals"];

/**
 * Everything moving on the map, in one request.
 *
 * Only the layers actually being shown are asked for. On a blood moon the
 * zombie list runs to hundreds of entries, and fetching it every few seconds
 * for a layer nobody has switched on is the easiest way to make both the panel
 * and the game server work for nothing.
 *
 * Polling stops while the tab is in the background, which is react-query's
 * default and worth keeping: a map left open on a second monitor should not
 * poll a game server all night.
 */
export function useMovingMarkers(layers: MapLayerName[], everyMs = 5000) {
  const serverId = useServerId();
  const wanted = MOVING_LAYERS.filter((layer) => layers.includes(layer));

  return useQuery({
    queryKey: ["map-markers", serverId, wanted.join(",")],
    queryFn: () => api.mapMarkers(serverId, wanted),
    enabled: serverId !== "" && wanted.length > 0,
    refetchInterval: everyMs,
    // Hold the last good positions through a blip rather than emptying the map.
    placeholderData: (previous) => previous,
  });
}

/**
 * Land claims, which are asked for far less often.
 *
 * A claim only changes when somebody places or loses a block, which is rare
 * and not something anybody watches happen. Polling it at the rate players
 * move would multiply the cost of the map for no visible benefit.
 */
export function useClaimMarkers(enabled: boolean, everyMs = 120_000) {
  const serverId = useServerId();
  return useQuery({
    queryKey: ["map-claims", serverId],
    queryFn: () => api.mapMarkers(serverId, ["claims"]),
    enabled: serverId !== "" && enabled,
    refetchInterval: everyMs,
    staleTime: everyMs,
    placeholderData: (previous) => previous,
  });
}

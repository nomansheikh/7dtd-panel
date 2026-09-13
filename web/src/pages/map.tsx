import { useMemo, useState } from "react";
import { api, type MapLayerName, type MapMarker } from "@/lib/api";
import { useServerId } from "@/hooks/use-servers";
import { useTeleportToPoint } from "@/hooks/use-game-map";
import { useClaimMarkers, useMapConfig, useMovingMarkers } from "@/hooks/use-game-map";
import { useDashboard } from "@/hooks/use-dashboard";
import { MapCanvas } from "@/components/map/map-canvas";
import { MapLegend } from "@/components/map/map-legend";
import { MapPlayers } from "@/components/map/map-players";
import { MapMenu } from "@/components/map/map-menu";
import { MapFreshness, MapScale } from "@/components/map/map-aside";
import { MapEmpty, MapUnavailable } from "@/components/map/map-unavailable";
import { MapCrosshair, MapFullscreenButton, type MapPosition } from "@/components/map/map-controls";
import { useFullscreen } from "@/hooks/use-fullscreen";
import "@/components/map/map.css";

const DEFAULT_LAYERS: MapLayerName[] = ["players", "claims"];

/**
 * How often to look for newly drawn ground.
 *
 * While somebody is playing, the server is drawing: walking into country
 * nobody has visited renders it, and an operator watching the map expects to
 * see it appear. With nobody in the world almost nothing can change, so the
 * map stops asking and the button covers the rest.
 */
const REFRESH_WHILE_PLAYED = 15_000;
const REFRESH_WHILE_EMPTY = 0;

/**
 * The live map.
 *
 * The two halves are polled at different rates on purpose: players and
 * entities move and are read every few seconds, while land claims barely
 * change and are read every couple of minutes. Asking for all of it at the
 * fast rate would multiply the cost of the page for nothing anybody would see.
 */
export function MapPage() {
  const serverId = useServerId();
  const [shown, setShown] = useState<MapLayerName[]>(DEFAULT_LAYERS);
  const [position, setPosition] = useState<MapPosition | null>(null);
  // Undefined until the first screenful has settled, so the explanation does
  // not flash up while tiles are still on their way.
  const [anyTiles, setAnyTiles] = useState<boolean | undefined>(undefined);
  const [refreshNonce, setRefreshNonce] = useState(0);
  const [refreshing, setRefreshing] = useState(false);
  const [focus, setFocus] = useState<{ x: number; z: number; at: number } | null>(null);
  const [menuAt, setMenuAt] = useState<{ x: number; z: number } | null>(null);
  const teleport = useTeleportToPoint();

  // Full screen takes the whole page region, so the layer switches come with
  // it rather than being left behind on a screen nobody can see.
  const fullscreen = useFullscreen();

  const config = useMapConfig();
  const dashboard = useDashboard();
  const playing = (dashboard.data?.players.online ?? 0) > 0;
  const moving = useMovingMarkers(shown);
  const claims = useClaimMarkers(shown.includes("claims"));

  const markers = useMemo<Partial<Record<MapLayerName, MapMarker[]>>>(
    () => ({ ...moving.data?.layers, ...claims.data?.layers }),
    [moving.data, claims.data],
  );
  const problems = useMemo(
    () => ({ ...moving.data?.problems, ...claims.data?.problems }),
    [moving.data, claims.data],
  );
  const counts = useMemo(
    () =>
      Object.fromEntries(
        Object.entries(markers).map(([layer, found]) => [layer, found.length]),
      ) as Partial<Record<MapLayerName, number>>,
    [markers],
  );

  const toggle = (layer: MapLayerName, on: boolean) =>
    setShown((current) =>
      on ? [...current, layer] : current.filter((existing) => existing !== layer),
    );

  if (config.isLoading) {
    return <div className="h-full" aria-busy="true" />;
  }
  if (config.isError || !config.data) {
    return <MapUnavailable />;
  }
  if (!config.data.enabled) {
    return <MapUnavailable />;
  }

  return (
    <div className="flex h-full min-h-0 flex-col bg-background lg:flex-row">
      <div className="relative min-h-0 flex-1">
        <MapMenu
          at={menuAt}
          players={markers.players ?? []}
          onTeleport={(player, x, z) =>
            teleport.mutate({ entityId: player.id, name: player.name, x, z })
          }
        >
          <div className="h-full w-full">
            <MapCanvas
              config={config.data}
              tileTemplate={api.mapTileTemplate(serverId)}
              markers={markers}
              shown={shown}
              onPositionChange={setPosition}
              onTilesSeen={setAnyTiles}
              refreshMs={playing ? REFRESH_WHILE_PLAYED : REFRESH_WHILE_EMPTY}
              refreshNonce={refreshNonce}
              onRefreshingChange={setRefreshing}
              focus={focus}
              onContextMenu={setMenuAt}
            />
          </div>
        </MapMenu>
        {anyTiles === false ? <MapEmpty /> : null}
        <MapCrosshair position={position} />
        {fullscreen.supported ? (
          <MapFullscreenButton
            isFullscreen={fullscreen.isFullscreen}
            onToggle={fullscreen.toggle}
          />
        ) : null}
      </div>

      <aside className="shrink-0 divide-y divide-border overflow-y-auto border-t border-border lg:w-72 lg:border-t-0 lg:border-l">
        <MapPlayers
          players={markers.players ?? []}
          hidden={!shown.includes("players")}
          onGoTo={(player) => setFocus({ x: player.x, z: player.z, at: Date.now() })}
        />
        <MapLegend shown={shown} counts={counts} problems={problems} onToggle={toggle} />
        <MapFreshness
          status={dashboard.data?.status ?? "unknown"}
          playing={playing}
          refreshing={refreshing}
          onRefresh={() => setRefreshNonce(Date.now())}
        />
        <MapScale worldSize={config.data.worldSize} maxZoom={config.data.maxZoom} />
      </aside>
    </div>
  );
}

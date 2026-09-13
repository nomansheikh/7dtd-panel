import { useMemo, useState } from "react";
import { RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { api, type MapLayerName, type MapMarker } from "@/lib/api";
import { useServerId } from "@/hooks/use-servers";
import { useClaimMarkers, useMapConfig, useMovingMarkers } from "@/hooks/use-game-map";
import { useDashboard } from "@/hooks/use-dashboard";
import { MapCanvas } from "@/components/map/map-canvas";
import { MapLegend } from "@/components/map/map-legend";
import { MapEmpty, MapUnavailable } from "@/components/map/map-unavailable";
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
  const [position, setPosition] = useState<{ x: number; z: number; zoom: number } | null>(null);
  // Undefined until the first screenful has settled, so the explanation does
  // not flash up while tiles are still on their way.
  const [anyTiles, setAnyTiles] = useState<boolean | undefined>(undefined);
  const [refreshNonce, setRefreshNonce] = useState(0);

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
    <div className="flex h-full min-h-0 flex-col lg:flex-row">
      <div className="relative min-h-0 flex-1">
        <MapCanvas
          config={config.data}
          tileTemplate={api.mapTileTemplate(serverId)}
          markers={markers}
          shown={shown}
          onPositionChange={setPosition}
          onTilesSeen={setAnyTiles}
          refreshMs={playing ? REFRESH_WHILE_PLAYED : REFRESH_WHILE_EMPTY}
          refreshNonce={refreshNonce}
        />
        {anyTiles === false ? <MapEmpty /> : null}
        <MapCrosshair position={position} />
      </div>

      <aside className="shrink-0 overflow-y-auto border-ash-800 lg:w-72 lg:border-l">
        <MapLegend shown={shown} counts={counts} problems={problems} onToggle={toggle} />
        <MapFreshness playing={playing} onRefresh={() => setRefreshNonce(Date.now())} />
        <MapScale worldSize={config.data.worldSize} maxZoom={config.data.maxZoom} />
      </aside>
    </div>
  );
}

/**
 * Says how current the ground is, and offers to go and look again.
 *
 * The button is here rather than on the map because it is not a view control:
 * it fetches every square on screen afresh, ignoring every cache between here
 * and the game server. That is the right thing after running visitmap, and the
 * wrong thing to do on a timer.
 */
function MapFreshness({ playing, onRefresh }: { playing: boolean; onRefresh: () => void }) {
  return (
    <div className="region">
      <div className="region-head">Ground</div>
      <div className="space-y-2 p-3">
        <p className="text-2xs text-bone-faint">
          {playing
            ? "Somebody is playing, so the map checks for newly drawn ground every 15 seconds."
            : "Nobody is in the world, so nothing is being drawn and the map is not asking."}
        </p>
        <Button variant="outline" size="sm" className="w-full" onClick={onRefresh}>
          <RefreshCw aria-hidden />
          Look again now
        </Button>
        <p className="text-2xs text-bone-faint">
          Fetches every square on screen again, ignoring what the browser already has. Worth it
          after rendering the map from the console.
        </p>
      </div>
    </div>
  );
}

/** Where the middle of the view is, in the coordinates the game uses. */
function MapCrosshair({ position }: { position: { x: number; z: number; zoom: number } | null }) {
  if (!position) return null;
  return (
    <div className="pointer-events-none absolute bottom-2 left-2 z-[500] border border-ash-800 bg-ash-950/85 px-2 py-1">
      <span className="readout text-2xs text-bone-dim">
        {position.x} E · {position.z} N · zoom {position.zoom}
      </span>
    </div>
  );
}

function MapScale({ worldSize, maxZoom }: { worldSize: number; maxZoom: number }) {
  return (
    <div className="region">
      <div className="region-head">This world</div>
      <dl className="grid grid-cols-2 gap-x-3 gap-y-1 p-3 text-sm">
        <dt className="text-bone-dim">Width</dt>
        <dd className="readout text-right">{worldSize.toLocaleString()} blocks</dd>
        <dt className="text-bone-dim">Closest zoom</dt>
        <dd className="readout text-right">1 block per pixel</dd>
      </dl>
      <p className="px-3 pb-3 text-2xs text-bone-faint">
        At the closest zoom the whole world is {worldSize.toLocaleString()} pixels across, which
        is {((worldSize / 128) ** 2).toLocaleString()} squares. Only the ones on screen are ever
        fetched, and each is kept until the server has had a chance to redraw it.
      </p>
      <p className="px-3 pb-3 text-2xs text-bone-faint">Rendered to zoom {maxZoom}.</p>
    </div>
  );
}

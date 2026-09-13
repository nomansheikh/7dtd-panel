import { Switch } from "@/components/ui/switch";
import type { MapLayerName } from "@/lib/api";
import { cn } from "@/lib/utils";

interface LayerSpec {
  name: MapLayerName;
  label: string;
  swatch: string;
  /** Why an operator would turn it on, and what it costs to leave it on. */
  note: string;
}

/**
 * The overlays, in the order they matter.
 *
 * Zombies and animals are off by default and say why: they only exist in
 * chunks the server has loaded, so on a quiet server the layer is empty, and
 * on a blood moon it is hundreds of entries re-read every few seconds.
 */
export const LAYERS: LayerSpec[] = [
  {
    name: "players",
    label: "Players",
    swatch: "bg-bone",
    note: "Everybody online, where they are standing",
  },
  {
    name: "claims",
    label: "Land claims",
    swatch: "bg-ember",
    note: "The square each claim block protects. Read every two minutes",
  },
  {
    name: "hostiles",
    label: "Zombies",
    swatch: "bg-crimson",
    note: "Only those in loaded chunks. Hundreds on a blood moon",
  },
  {
    name: "animals",
    label: "Animals",
    swatch: "bg-ember/60",
    note: "Only those in loaded chunks",
  },
];

interface MapLegendProps {
  shown: MapLayerName[];
  counts: Partial<Record<MapLayerName, number>>;
  problems: Partial<Record<MapLayerName, string>>;
  onToggle: (layer: MapLayerName, on: boolean) => void;
}

/** What is drawn on the map, and the switch for each. */
export function MapLegend({ shown, counts, problems, onToggle }: MapLegendProps) {
  return (
    <div className="region">
      <div className="region-head gap-2">
        <span className="map-lead" aria-hidden />
        <span className="stencil">Layers</span>
      </div>
      <ul>
        {LAYERS.map((layer) => {
          const on = shown.includes(layer.name);
          const problem = problems[layer.name];
          return (
            <li key={layer.name} className="border-t border-border px-4 py-2 first:border-t-0">
              <div className="flex items-center gap-2">
                <span className="map-lead">
                  <span className={cn("size-2", layer.swatch)} aria-hidden />
                </span>
                <span className="flex-1 text-sm">{layer.label}</span>
                {on && !problem ? (
                  <span className="readout text-2xs text-bone-dim">{counts[layer.name] ?? 0}</span>
                ) : null}
                <Switch
                  checked={on}
                  onCheckedChange={(next) => onToggle(layer.name, next)}
                  aria-label={`Show ${layer.label.toLowerCase()} on the map`}
                />
              </div>
              <p className="mt-1 pl-6 text-2xs text-bone-faint">{layer.note}</p>
              {on && problem ? (
                <p className="mt-1 pl-6 text-2xs text-crimson-lit">Could not be read: {problem}</p>
              ) : null}
            </li>
          );
        })}
      </ul>
    </div>
  );
}

import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Minus, Plus } from "lucide-react";
import { api } from "@/lib/api";
import { useServerId } from "@/hooks/use-servers";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  InputGroup,
  InputGroupAddon,
  InputGroupButton,
  InputGroupInput,
} from "@/components/ui/input-group";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { cn } from "@/lib/utils";

export interface SpawnAt {
  x: number;
  z: number;
  /** The height to start from, taken from the nearest thing the panel can see. */
  suggestedY: number;
  /** Whether that height came from something real or is only a guess. */
  heightIsGuessed: boolean;
}

interface MapSpawnProps {
  at: SpawnAt | null;
  onClose: () => void;
  onSpawn: (entity: string, y: number, count: number) => void;
}

const stepper = "h-7 w-28 shrink-0";

/*
Spawning at a point on the map, which needs a height the map does not have.

teleportplayer takes y = -1 and finds the ground. spawnentityat takes the same
argument, treats it as a literal height and throws the entity away while still
answering "Spawned 1" — measured against a live server: three at y = -1 produced
none, three at y = 64 produced three. No console command reports terrain height,
and an entity spawned into an unsimulated chunk does not fall to find it: one
left at y = 150 was still at y = 150 twelve seconds later.

So the height is asked for, and what a wrong one does is spelled out.
*/
export function MapSpawn({ at, onClose, onSpawn }: MapSpawnProps) {
  const serverId = useServerId();
  const [query, setQuery] = useState("");
  const [chosen, setChosen] = useState("");
  const [count, setCount] = useState(1);
  const [height, setHeight] = useState<number | null>(null);

  const { data, isLoading } = useQuery({
    queryKey: ["entities", serverId],
    queryFn: () => api.spawnableEntities(serverId),
    enabled: serverId !== "" && at !== null,
    staleTime: 10 * 60 * 1000,
  });

  const matches = useMemo(() => {
    const all = data?.entities ?? [];
    const needle = query.trim().toLowerCase();
    const found = needle ? all.filter((e) => e.name.toLowerCase().includes(needle)) : all;
    return found.slice(0, 200);
  }, [data, query]);

  const y = height ?? at?.suggestedY ?? 64;

  return (
    <Sheet open={at !== null} onOpenChange={(open) => !open && onClose()}>
      <SheetContent className="flex w-full flex-col sm:max-w-xl">
        <SheetHeader className="region-head shrink-0 space-y-0 p-3 md:px-4">
          <SheetTitle className="stencil">Spawn here</SheetTitle>
          <SheetDescription className="readout text-2xs text-bone-dim">
            x {at ? Math.round(at.x) : 0} · z {at ? Math.round(at.z) : 0}
          </SheetDescription>
        </SheetHeader>

        <div className="min-h-0 flex-1 space-y-3 overflow-y-auto p-4">
          <Input
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="Search the spawnable classes"
            aria-label="Search the spawnable classes"
          />

          {isLoading ? (
            <p className="text-2xs text-bone-faint">Reading the catalogue…</p>
          ) : (
            <ul className="max-h-72 divide-y divide-border overflow-y-auto border border-border">
              {matches.map((entity) => (
                <li key={entity.name}>
                  <button
                    type="button"
                    onClick={() => setChosen(entity.name)}
                    className={cn(
                      "flex w-full items-center px-3 py-1.5 text-left text-xs",
                      chosen === entity.name ? "bg-accent text-bone" : "hover:bg-accent/50",
                    )}
                  >
                    <span className="readout truncate">{entity.name}</span>
                  </button>
                </li>
              ))}
              {matches.length === 0 ? (
                <li className="px-3 py-2 text-2xs text-bone-faint">
                  Nothing matches. The list is the game's own spawnable classes.
                </li>
              ) : null}
            </ul>
          )}

          <div className="flex flex-wrap items-center gap-3">
            <Stepper
              label="How many to spawn"
              value={count}
              onChange={(next) => setCount(Math.max(1, Math.min(50, next)))}
            />
            <Stepper
              label="What height to spawn at"
              value={y}
              onChange={(next) => setHeight(Math.max(0, Math.min(255, next)))}
            />
          </div>

          <p className="text-2xs text-bone-faint">
            The game will not find the ground for you. Below the terrain the spawn is lost with no
            error; above it the entity hangs in the air until somebody loads the chunk.
            {at?.heightIsGuessed
              ? " Nothing is loaded near here, so this height is a guess."
              : " This height came from the nearest thing the panel can see."}
          </p>
        </div>

        <SheetFooter className="shrink-0 border-t border-border p-4">
          <Button
            disabled={!chosen}
            onClick={() => {
              onSpawn(chosen, y, count);
              onClose();
            }}
          >
            {chosen ? `Spawn ${count} × ${chosen}` : "Pick something to spawn"}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  );
}

function Stepper({
  label,
  value,
  onChange,
}: {
  label: string;
  value: number;
  onChange: (next: number) => void;
}) {
  return (
    <InputGroup className={stepper}>
      <InputGroupAddon align="inline-start">
        <InputGroupButton
          size="icon-xs"
          onClick={() => onChange(value - 1)}
          aria-label={`Fewer: ${label}`}
        >
          <Minus className="size-3" />
        </InputGroupButton>
      </InputGroupAddon>
      <InputGroupInput
        inputMode="numeric"
        value={String(value)}
        onChange={(event) => onChange(Number(event.target.value) || 0)}
        className="readout h-7 px-1 text-center text-xs"
        aria-label={label}
      />
      <InputGroupAddon align="inline-end">
        <InputGroupButton
          size="icon-xs"
          onClick={() => onChange(value + 1)}
          aria-label={`More: ${label}`}
        >
          <Plus className="size-3" />
        </InputGroupButton>
      </InputGroupAddon>
    </InputGroup>
  );
}

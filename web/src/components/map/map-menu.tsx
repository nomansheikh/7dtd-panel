import { useState, type ReactNode } from "react";
import { Check, Copy, MapPin } from "lucide-react";
import type { MapMarker } from "@/lib/api";
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuLabel,
  ContextMenuSeparator,
  ContextMenuSub,
  ContextMenuSubContent,
  ContextMenuSubTrigger,
  ContextMenuTrigger,
} from "@/components/ui/context-menu";
import { compass } from "@/components/map/tooltips";

interface MapMenuProps {
  /** Where the last right-click landed, in the game's own axes. */
  at: { x: number; z: number } | null;
  players: MapMarker[];
  onTeleport: (player: MapMarker, x: number, z: number) => void;
  children: ReactNode;
}

/*
Right-clicking a place is the only way to act on a point: every console command
that takes a position takes three numbers nobody wants to read off a screen and
retype.

Teleport is offered because the game finds the ground itself — teleportplayer
documents "use y = -1 to spawn on ground". Spawning is not: spawnentityat takes
the same three numbers, treats -1 as a literal height and discards the entity
while still answering "Spawned 1", so it needs a height the map does not have.

Leaflet calls preventDefault on contextmenu but never stopPropagation, so the
event still reaches this trigger while the native menu stays suppressed.
*/
export function MapMenu({ at, players, onTeleport, children }: MapMenuProps) {
  const [copied, setCopied] = useState(false);

  const x = Math.round(at?.x ?? 0);
  const z = Math.round(at?.z ?? 0);
  const forCommand = `${x} -1 ${z}`;

  return (
    <ContextMenu onOpenChange={(open) => open && setCopied(false)}>
      <ContextMenuTrigger asChild>{children}</ContextMenuTrigger>
      <ContextMenuContent className="w-56">
        <ContextMenuLabel className="flex items-center gap-2">
          <MapPin className="size-3.5 shrink-0 text-bone-faint" aria-hidden />
          <span className="readout text-2xs text-bone">{compass(x, z)}</span>
        </ContextMenuLabel>
        <ContextMenuSeparator />

        <ContextMenuSub>
          <ContextMenuSubTrigger disabled={players.length === 0}>
            {players.length === 0 ? "Nobody to teleport" : "Teleport somebody here"}
          </ContextMenuSubTrigger>
          <ContextMenuSubContent>
            {players.map((player) => (
              <ContextMenuItem key={player.id} onSelect={() => onTeleport(player, x, z)}>
                {player.name}
              </ContextMenuItem>
            ))}
          </ContextMenuSubContent>
        </ContextMenuSub>

        <ContextMenuItem
          onSelect={(event) => {
            event.preventDefault();
            void navigator.clipboard?.writeText(forCommand);
            setCopied(true);
          }}
        >
          {copied ? (
            <Check className="text-ember" aria-hidden />
          ) : (
            <Copy className="text-bone-faint" aria-hidden />
          )}
          {copied ? "Copied" : "Copy for a command"}
        </ContextMenuItem>

        <ContextMenuSeparator />
        <p className="px-2 py-1.5 text-2xs text-bone-faint">
          <span className="readout">{forCommand}</span> — the -1 tells the game to put it on the
          ground.
        </p>
      </ContextMenuContent>
    </ContextMenu>
  );
}

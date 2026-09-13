import { useEffect, useRef, useState, type ReactNode } from "react";
import { Check, ChevronRight, Copy, MapPin } from "lucide-react";
import type { MapMarker } from "@/lib/api";
import { compass } from "@/components/map/tooltips";
import { cn } from "@/lib/utils";

export interface MapMenuAt {
  /** Where on the page to draw it. */
  screenX: number;
  screenY: number;
  /** Where in the world it points at. The map knows no height. */
  x: number;
  z: number;
}

interface MapMenuProps {
  at: MapMenuAt | null;
  players: MapMarker[];
  onClose: () => void;
  onTeleport: (player: MapMarker, x: number, z: number) => void;
}

/*
Right-clicking a place is the only way to act on a point: every console command
that takes a position takes three numbers nobody wants to read off a screen and
retype.

Teleport is offered because the game will find the ground itself — teleportplayer
documents "use y = -1 to spawn on ground". Spawning is not, yet: spawnentityat
takes the same y and silently drops the entity when it is -1, reporting success,
so a spawn from here needs a height the map does not have.
*/
export function MapMenu({ at, players, onClose, onTeleport }: MapMenuProps) {
  const holder = useRef<HTMLDivElement | null>(null);
  const [submenu, setSubmenu] = useState(false);
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    setSubmenu(false);
    setCopied(false);
  }, [at]);

  useEffect(() => {
    if (!at) return;
    const away = (event: MouseEvent) => {
      if (!holder.current?.contains(event.target as Node)) onClose();
    };
    const escape = (event: KeyboardEvent) => {
      if (event.key === "Escape") onClose();
    };
    document.addEventListener("mousedown", away);
    document.addEventListener("keydown", escape);
    return () => {
      document.removeEventListener("mousedown", away);
      document.removeEventListener("keydown", escape);
    };
  }, [at, onClose]);

  if (!at) return null;

  const coordinates = `${Math.round(at.x)} -1 ${Math.round(at.z)}`;
  /* Flipped near the edges so the menu never opens off screen. */
  const nearRight = at.screenX > window.innerWidth - 240;
  const nearBottom = at.screenY > window.innerHeight - 260;

  return (
    <div
      ref={holder}
      role="menu"
      style={{
        left: nearRight ? undefined : at.screenX,
        right: nearRight ? window.innerWidth - at.screenX : undefined,
        top: nearBottom ? undefined : at.screenY,
        bottom: nearBottom ? window.innerHeight - at.screenY : undefined,
      }}
      className="fixed z-[1000] w-56 border border-border bg-background"
    >
      <div className="flex items-center gap-2 border-b border-border px-3 py-2">
        <MapPin className="size-3.5 shrink-0 text-bone-faint" aria-hidden />
        <span className="readout text-2xs text-bone">{compass(at.x, at.z)}</span>
      </div>

      <div className="relative">
        <MenuItem
          onClick={() => setSubmenu((was) => !was)}
          disabled={players.length === 0}
          trailing={<ChevronRight className="size-3.5" aria-hidden />}
        >
          {players.length === 0 ? "Nobody to teleport" : "Teleport somebody here"}
        </MenuItem>

        {submenu && players.length > 0 ? (
          <div className="border-t border-border bg-accent/40">
            {players.map((player) => (
              <MenuItem
                key={player.id}
                indent
                onClick={() => {
                  onTeleport(player, Math.round(at.x), Math.round(at.z));
                  onClose();
                }}
              >
                {player.name}
              </MenuItem>
            ))}
          </div>
        ) : null}
      </div>

      <MenuItem
        onClick={() => {
          void navigator.clipboard?.writeText(coordinates);
          setCopied(true);
        }}
        trailing={
          copied ? (
            <Check className="size-3.5 text-ember" aria-hidden />
          ) : (
            <Copy className="size-3.5" aria-hidden />
          )
        }
      >
        {copied ? "Copied" : "Copy for a command"}
      </MenuItem>

      <p className="border-t border-border px-3 py-2 text-2xs text-bone-faint">
        Copies <span className="readout">{coordinates}</span>. The -1 tells the game to put it on
        the ground.
      </p>
    </div>
  );
}

function MenuItem({
  children,
  onClick,
  disabled,
  trailing,
  indent,
}: {
  children: ReactNode;
  onClick: () => void;
  disabled?: boolean;
  trailing?: ReactNode;
  indent?: boolean;
}) {
  return (
    <button
      type="button"
      role="menuitem"
      disabled={disabled}
      onClick={onClick}
      className={cn(
        "flex w-full items-center gap-2 px-3 py-2 text-left text-sm",
        indent && "pl-7 text-bone-dim",
        disabled ? "text-bone-faint" : "hover:bg-accent hover:text-bone",
      )}
    >
      <span className="flex-1 truncate">{children}</span>
      {trailing ? <span className="shrink-0 text-bone-faint">{trailing}</span> : null}
    </button>
  );
}

import { useState } from "react";
import { Link } from "react-router-dom";
import { ChevronDown, ChevronRight, Crosshair, UserRound } from "lucide-react";
import type { MapMarker } from "@/lib/api";
import { cn } from "@/lib/utils";

interface MapPlayersProps {
  players: MapMarker[];
  /** True while the player layer is switched off, which is why the list is empty. */
  hidden: boolean;
  onGoTo: (player: MapMarker) => void;
}

/**
 * Who is online, and a way to go and look at them.
 *
 * Finding one player on a 6144-block map by panning is not realistic — a
 * marker is a few pixels and most of the world is off screen. The list is the
 * index into the map.
 *
 * Two different destinations, so they are two different controls rather than
 * one that guesses: the row moves the map to where they are standing, and the
 * name opens their page. Clicking a name to be silently panned somewhere else
 * is the kind of thing that makes people stop trusting a control.
 */
export function MapPlayers({ players, hidden, onGoTo }: MapPlayersProps) {
  const [open, setOpen] = useState(true);
  const Chevron = open ? ChevronDown : ChevronRight;

  return (
    <div className="region">
      <button
        type="button"
        onClick={() => setOpen((was) => !was)}
        aria-expanded={open}
        className="region-head w-full text-left"
      >
        <Chevron className="size-3.5 shrink-0 text-bone-faint" aria-hidden />
        <span className="stencil flex-1">Online</span>
        <span className="readout text-2xs text-bone-dim">{players.length}</span>
      </button>

      {open ? (
        players.length === 0 ? (
          <p className="px-4 py-3 text-2xs text-bone-faint">
            {hidden
              ? "The player layer is switched off, so nobody is being fetched."
              : "Nobody is in the world. Anyone who joins appears here and on the map."}
          </p>
        ) : (
          <ul>
            {players.map((player) => (
              <li
                key={player.id}
                className="flex items-center gap-1 border-t border-border first:border-t-0"
              >
                <Link
                  to={`/players/${encodeURIComponent(player.platformId ?? "")}`}
                  className="flex min-w-0 flex-1 items-center gap-2 px-4 py-2 hover:bg-accent"
                >
                  <UserRound className="size-3.5 shrink-0 text-bone-faint" aria-hidden />
                  <span className="truncate text-sm">{player.name}</span>
                </Link>
                <button
                  type="button"
                  onClick={() => onGoTo(player)}
                  aria-label={`Move the map to ${player.name}`}
                  className={cn(
                    "mr-2 flex size-7 shrink-0 items-center justify-center",
                    "text-bone-faint hover:bg-accent hover:text-bone",
                  )}
                >
                  <Crosshair className="size-3.5" aria-hidden />
                </button>
              </li>
            ))}
          </ul>
        )
      ) : null}
    </div>
  );
}

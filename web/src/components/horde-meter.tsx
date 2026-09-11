import { cn } from "@/lib/utils";
import { bloodMoonProgress } from "@/lib/format";

interface Props {
  currentDay: number;
  nextDay: number;
  active: boolean;
  /** compact is the sidebar's; full is the dashboard's. */
  size?: "compact" | "full";
  className?: string;
}

/**
 * The seven-day cycle, drawn as seven segments.
 *
 * Seven is the game's own unit, not a design choice: a bar that filled
 * smoothly would be prettier and would tell you less, because what an operator
 * actually wants to know is how many nights are left. Days already survived
 * are crimson, the night in progress pulses, and the rest are ash.
 *
 * Deliberately not a shadcn Progress: that renders one continuous track, and
 * the whole point here is that the unit is discrete.
 */
export function HordeMeter({ currentDay, nextDay, active, size = "full", className }: Props) {
  const { daysAway, elapsed, total } = bloodMoonProgress(currentDay, nextDay);
  const filled = active ? total : elapsed;

  return (
    <div
      className={cn("flex", size === "full" ? "gap-1.5" : "gap-1", className)}
      role="meter"
      aria-valuemin={0}
      aria-valuemax={total}
      aria-valuenow={filled}
      aria-label={
        active ? "Blood moon tonight" : `${daysAway} of ${total} days until the blood moon`
      }
    >
      {Array.from({ length: total }, (_, i) => {
        const spent = i < filled;
        const current = !active && i === filled;
        return (
          <span
            key={i}
            className={cn(
              "flex-1 transition-colors duration-700",
              size === "full" ? "h-2" : "h-1",
              spent && "bg-crimson",
              current && "animate-breathe bg-crimson-deep",
              // Nights still to come are empty slots rather than dark fill, so
              // the meter reads as "five of seven" even against a lit panel.
              !spent && !current && "bg-bone/12",
            )}
            style={
              // The last segment is the night itself, so it burns brighter than
              // the days that led to it.
              spent && i === total - 1
                ? { background: "var(--crimson-lit)", boxShadow: "0 0 14px var(--crimson-lit)" }
                : undefined
            }
          />
        );
      })}
    </div>
  );
}

import { Moon } from "lucide-react";
import { cn } from "@/lib/utils";
import { useDashboard } from "@/hooks/use-dashboard";
import { bloodMoonProgress, formatGameClock } from "@/lib/format";

/**
 * The seven nights, full width, under the top bar on every page.
 *
 * This is the panel's spine. The game is a countdown and nothing else, so the
 * countdown is the one thing that never leaves the screen — not tucked in a
 * card on the overview where you have to go and look for it.
 *
 * Each cell is a real in-game day with its own number, which is what makes it
 * worth the space: you can read "the horde lands on day 14" off it, not just
 * "five sevenths of the way through something". The night itself is the last
 * cell and it burns.
 */
export function CycleStrip() {
  const { data } = useDashboard();
  if (!data?.bloodMoon) return null;

  const { world, bloodMoon } = data;
  const { daysAway, total } = bloodMoonProgress(world.day, bloodMoon.nextDay);
  // The cycle's first day is however many days back the last horde was.
  const firstDay = bloodMoon.nextDay - total + 1;
  // Which cell is today. Derived from the day number rather than from the
  // elapsed count, which counts today as already spent and so lit the cell for
  // tomorrow.
  const today = world.day - firstDay;

  return (
    <div className="flex shrink-0 items-stretch border-b border-border" aria-hidden>
      <div className="hidden w-40 shrink-0 items-center gap-2 border-r border-border px-4 sm:flex md:w-52 md:px-6">
        <Moon
          className={cn(
            "size-3.5 shrink-0",
            bloodMoon.active ? "animate-breathe text-crimson-lit" : "text-bone-faint",
          )}
        />
        <span className={cn("stencil", bloodMoon.active && "text-crimson-lit")}>
          {bloodMoon.active ? "Tonight" : daysAway === 1 ? "1 day out" : `${daysAway} days out`}
        </span>
      </div>

      <ol className="flex min-w-0 flex-1">
        {Array.from({ length: total }, (_, i) => {
          const day = firstDay + i;
          const night = i === total - 1;
          const spent = bloodMoon.active ? true : i < today;
          const now = !bloodMoon.active && i === today;

          return (
            <li
              key={day}
              className={cn(
                "relative flex min-w-0 flex-1 flex-col justify-center gap-1.5 border-r border-border px-2 py-2 last:border-r-0",
                now && "bg-bone/4",
              )}
            >
              <span
                className={cn(
                  "readout truncate text-center text-2xs leading-none",
                  night ? "text-crimson-lit" : now ? "text-bone" : "text-bone-faint",
                )}
              >
                {night ? "HORDE" : day}
              </span>
              <span
                className={cn(
                  "h-1 w-full transition-colors duration-700",
                  spent && night && "bg-crimson-lit shadow-[0_0_12px_var(--crimson-lit)]",
                  spent && !night && "bg-crimson",
                  now && "animate-breathe bg-crimson-deep",
                  !spent && !now && night && "bg-crimson-deep",
                  !spent && !now && !night && "bg-bone/12",
                )}
              />
            </li>
          );
        })}
      </ol>

      <div className="hidden w-28 shrink-0 items-center justify-end border-l border-border px-4 md:flex">
        <span className="readout text-sm text-bone-dim">
          {formatGameClock(world.hour, world.minute)}
        </span>
      </div>
    </div>
  );
}

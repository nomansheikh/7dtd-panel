import { cn } from "@/lib/utils";
import { bloodMoonProgress } from "@/lib/format";

interface Props {
  currentDay: number;
  nextDay: number;
  nextHour: number;
  active: boolean;
}

/**
 * The blood moon panel.
 *
 * This is the only saturated colour in the interface, and its intensity rises
 * as the horde approaches, so the colour itself carries the information. The
 * game is structured entirely around this countdown, which is why it gets the
 * prominence rather than a generic status tile.
 */
export function BloodMoon({ currentDay, nextDay, nextHour, active }: Props) {
  const { daysAway, elapsed, total } = bloodMoonProgress(currentDay, nextDay);
  const imminent = daysAway <= 1;

  return (
    <section
      aria-label="Blood moon"
      className={cn(
        "relative overflow-hidden rounded-md border p-5",
        active || imminent ? "border-blood/70 bg-blood/10" : "border-border bg-card",
      )}
    >
      {/* A glow that grows with proximity, rather than a decorative gradient. */}
      <div
        aria-hidden
        className="pointer-events-none absolute -top-16 right-0 size-48 rounded-full blur-3xl"
        style={{
          background: "var(--blood)",
          opacity: active ? 0.4 : Math.max(0.04, (elapsed / total) * 0.22),
        }}
      />

      <div className="relative">
        <h2 className="text-sm font-medium text-muted-foreground">Blood moon</h2>

        {active ? (
          <p className="mt-2 text-3xl font-semibold text-blood-bright readout">Tonight, now</p>
        ) : (
          <p className="mt-2 flex items-baseline gap-2">
            <span
              className={cn(
                "text-5xl font-semibold readout",
                imminent ? "text-blood-bright" : "text-foreground",
              )}
            >
              {daysAway}
            </span>
            <span className="text-base text-muted-foreground">
              {daysAway === 1 ? "day away" : "days away"}
            </span>
          </p>
        )}

        <p className="mt-1 text-sm text-muted-foreground readout">
          Day {nextDay} at {String(nextHour).padStart(2, "0")}:00
        </p>

        {/* Seven ticks, one per day of the cycle: the game's own unit. */}
        <div className="mt-4 flex gap-1" aria-hidden>
          {Array.from({ length: total }, (_, i) => (
            <span
              key={i}
              className={cn("h-1.5 flex-1 rounded-full", i < elapsed ? "bg-blood" : "bg-muted")}
            />
          ))}
        </div>
      </div>
    </section>
  );
}

import { cn } from "@/lib/utils";
import type { SkyConditions } from "@/components/sky";

/**
 * The weather, drawn as the sky it is.
 *
 * The panel could set the weather but never show it, so six numbers in a table
 * were the whole picture: an operator changed the sky blind and then alt-tabbed
 * into the game to find out what they had done. Everything here comes from the
 * server — cloud is the biome's own thickness, rain and snow are its rates, the
 * fall leans with its wind, fog is a veil over the lot, the horizon warms with
 * its temperature, and the light is the world clock. Nothing is decoration.
 *
 * Lighting it from the clock is what turns it from a diagram into a sky. The
 * same eighteen-hour day the dial draws as a ring is drawn here as the thing
 * the ring is about: the sun crosses left to right, the horizon burns at dawn
 * and dusk, and after dark the moon comes up in its place — red on the night
 * it matters.
 */

export const W = 480;
export const H = 300;
export const HORIZON = 246;

/** A repeatable scatter, so the sky does not reshuffle itself on every render. */
function scatter(seed: number, count: number): number[] {
  const out: number[] = [];
  let n = seed;
  for (let i = 0; i < count; i++) {
    n = (n * 1103515245 + 12345) % 2147483648;
    out.push(n / 2147483648);
  }
  return out;
}

/**
 * A ragged treeline, drawn once and never animated.
 *
 * Built from runs of two or three conifers of a similar height rather than
 * from a spike per point: evenly alternating tall and short read as a sawtooth
 * border, which is what it looked like before.
 */
export const TREES: { x: number; w: number; h: number }[] = (() => {
  const r = scatter(7, 400);
  const out: { x: number; w: number; h: number }[] = [];
  let x = -24;
  let i = 0;
  while (x < W + 24) {
    // A stand shares a rough height, so the ridge has clumps in it rather than
    // an even comb.
    const stand = 12 + r[i % r.length] * 26;
    const trees = 4 + Math.floor(r[(i + 1) % r.length] * 5);
    for (let t = 0; t < trees && x < W + 24; t++) {
      const w = 16 + r[(i + t + 2) % r.length] * 26;
      const h = stand * (0.45 + r[(i + t + 3) % r.length] * 0.8);
      out.push({ x, w, h });
      // Advanced by well under a tree's own width, so neighbours overlap into
      // a single dark mass instead of standing apart like fence posts.
      x += w * (0.2 + r[(i + t + 5) % r.length] * 0.22);
    }
    i += trees + 4;
  }
  return out;
})();

/** The stand of trees as one shape, for either sky to use. */
export function Trees({ fill = "var(--ash-950)" }: { fill?: string }) {
  return (
    <g>
      {TREES.map((t, i) => (
        <polygon
          key={i}
          points={`${t.x},${HORIZON} ${t.x + t.w / 2},${HORIZON - t.h} ${t.x + t.w},${HORIZON}`}
          fill={fill}
        />
      ))}
      <rect x="0" y={HORIZON - 1} width={W} height={H - HORIZON + 1} fill={fill} />
    </g>
  );
}

interface Props {
  conditions: SkyConditions;
  /** The world clock, which is what actually lights the sky. */
  hour: number;
  dawnHour: number;
  daylightHours: number;
  hordeTonight?: boolean;
  className?: string;
}

export function SkyPanel({
  conditions,
  hour,
  dawnHour,
  daylightHours,
  hordeTonight,
  className,
}: Props) {
  const { cloud, rain, snow, fog, wind } = conditions;

  const duskHour = dawnHour + daylightHours;
  const day = hour >= dawnHour && hour < duskHour;

  // How far through its own arc the sun or the moon is, nought at one horizon
  // and one at the other. Both cross left to right, as on the dial.
  const through = day
    ? (hour - dawnHour) / daylightHours
    : ((hour < dawnHour ? hour + 24 : hour) - duskHour) / (24 - daylightHours);
  const height = Math.sin(Math.PI * through);

  // Only the sun lights the sky. Cloud takes some of it back.
  const light = day ? height * (1 - cloud * 0.55) : 0;
  // Low sun burns the horizon; high sun does not. This is the whole reason
  // dusk looks like anything at all.
  const burn = day ? Math.max(0, 1 - height * 2.2) : 0;

  const warmth = Math.min(1, Math.max(0, (conditions.temperature - 20) / 75));

  const zenith = `color-mix(in oklab, var(--ash-950), var(--ash-780) ${(light * 55).toFixed(0)}%)`;
  const horizon = `color-mix(in oklab, color-mix(in oklab, var(--ash-900), var(--ash-700) ${(light * 45).toFixed(0)}%), var(--ember) ${(burn * 42 + warmth * 10).toFixed(0)}%)`;

  const lean = wind * 34;
  const storming = conditions.state !== "default";

  const drops = scatter(11, Math.round(rain * 90));
  const flakes = scatter(23, Math.round(snow * 70));
  const clouds = scatter(31, 7);

  // The disc rides its own arc across the upper sky.
  const bodyX = 40 + through * (W - 80);
  const bodyY = HORIZON - 26 - height * (HORIZON - 96);

  return (
    <div className={cn("relative w-full", className)}>
      <svg
        viewBox={`0 0 ${W} ${H}`}
        className="w-full"
        role="img"
        aria-label={describe(conditions, day)}
      >
        <defs>
          <linearGradient id="sky-air" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor={zenith} />
            <stop offset="100%" stopColor={horizon} />
          </linearGradient>
          {/*
            Fog sits on the ground. A flat scrim over the whole frame just
            greyed the picture out; weighting it to the horizon is what makes
            it read as fog rather than as a dimmed screen.
          */}
          <linearGradient id="sky-fog" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor="var(--bone)" stopOpacity={0} />
            <stop offset="55%" stopColor="var(--bone)" stopOpacity={fog * 0.16} />
            <stop offset="100%" stopColor="var(--bone)" stopOpacity={fog * 0.42} />
          </linearGradient>
          <clipPath id="sky-air-clip">
            <rect x="0" y="0" width={W} height={HORIZON} />
          </clipPath>
        </defs>

        <rect x="0" y="0" width={W} height={H} fill="url(#sky-air)" />

        <g clipPath="url(#sky-air-clip)">
          {/* The sun, or the moon in its place. Dimmed by cloud rather than
              hidden, which is what thick cloud actually does to a disc. */}
          <g opacity={0.25 + (1 - cloud) * 0.75}>
            <circle
              cx={bodyX}
              cy={bodyY}
              r={day ? 15 : 11}
              fill={day ? "var(--ember)" : hordeTonight ? "var(--crimson-lit)" : "var(--bone-dim)"}
              style={{
                filter: `drop-shadow(0 0 ${day ? 34 : 20}px ${
                  day ? "var(--ember)" : hordeTonight ? "var(--crimson-lit)" : "var(--bone-faint)"
                })`,
              }}
            />
            {/* A crescent, cut by lifting a disc of the sky back over it. */}
            {!day && (
              <circle cx={bodyX + 6} cy={bodyY - 4} r={10} fill="url(#sky-air)" opacity={0.92} />
            )}
          </g>

          {/* Cloud, as banks rather than a flat wash: thickness decides how
              many are visible and how low they hang. */}
          {clouds.map((r, i) => {
            const depth = i / clouds.length;
            if (cloud <= depth * 0.85) return null;
            return (
              <ellipse
                key={i}
                cx={r * W}
                cy={18 + depth * 96}
                rx={70 + r * 110}
                ry={16 + r * 16}
                fill="var(--bone)"
                opacity={(0.05 + cloud * 0.17) * (1 - depth * 0.45)}
              />
            );
          })}

          {/*
            Rain and snow are drawn twice, one copy a full height above the
            other, and the pair is slid down by exactly that height on a loop.
            As the lower copy leaves the frame the upper one has taken its
            place, so the fall is continuous without a seam to spot.
          */}
          {rain > 0 && (
            <g className="sky-fall" style={{ animationDuration: `${1.6 - wind}s` }}>
              {[0, 1].map((copy) =>
                drops.map((r, i) => {
                  const x = r * (W + 120) - 60;
                  const y = scatter(i + 3, 1)[0] * HORIZON - copy * HORIZON;
                  return (
                    <line
                      key={`${copy}-${i}`}
                      x1={x}
                      y1={y}
                      x2={x - lean * 0.45}
                      y2={y + 15}
                      stroke="var(--bone)"
                      strokeOpacity={0.16 + rain * 0.22}
                      strokeWidth={1}
                    />
                  );
                }),
              )}
            </g>
          )}

          {snow > 0 && (
            <g className="sky-fall" style={{ animationDuration: `${7 - wind * 3}s` }}>
              {[0, 1].map((copy) =>
                flakes.map((r, i) => {
                  const x = r * (W + 120) - 60;
                  const y = scatter(i + 5, 1)[0] * HORIZON - copy * HORIZON;
                  return (
                    <circle
                      key={`${copy}-${i}`}
                      cx={x - lean * 0.3}
                      cy={y}
                      r={0.8 + r * 1.1}
                      fill="var(--bone)"
                      fillOpacity={0.3 + snow * 0.4}
                    />
                  );
                }),
              )}
            </g>
          )}
        </g>

        <Trees />

        {/* Fog last, over the treeline as well as the sky, because that is the
            order it takes them in. */}
        {fog > 0 && <rect x="0" y="0" width={W} height={H} fill="url(#sky-fog)" />}

        {storming && (
          <rect
            x="0"
            y="0"
            width={W}
            height={H}
            fill="var(--crimson)"
            opacity={0.07}
            className="animate-breathe"
          />
        )}
      </svg>
    </div>
  );
}

/** The same picture, for anyone who cannot see it. */
function describe(c: SkyConditions, day: boolean): string {
  const parts = [
    `${c.biome.replace(/_/g, " ")} by ${day ? "day" : "night"}, ${Math.round(c.temperature)} degrees`,
  ];
  if (c.rain > 0.05) parts.push("raining");
  if (c.snow > 0.05) parts.push("snowing");
  if (c.fog > 0.2) parts.push("foggy");
  if (c.cloud > 0.5) parts.push("overcast");
  if (c.wind > 0.2) parts.push("windy");
  if (c.state !== "default") parts.push(c.state);
  return parts.join(", ");
}

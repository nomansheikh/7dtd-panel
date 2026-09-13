import { useRef, useState } from "react";
import { cn } from "@/lib/utils";

/**
 * The day, drawn as the sky rather than as a clock.
 *
 * Noon sits at the top and midnight at the bottom, so the sun climbs the left
 * side, crosses the top and sets down the right: the ring is the path of the
 * light, and how much of it is lit is how long this world's day actually is.
 * An eighteen-hour day fills three quarters of the circle and the dark is the
 * wedge at the bottom, which is the whole game in one shape.
 *
 * A 00-at-top clock face would have been the obvious choice and would have
 * said nothing — it puts noon and midnight in the same place as every other
 * clock and leaves the reader to work out which half is dangerous. The seven
 * days are deliberately absent: the cycle strip above carries those on every
 * page, and drawing them again here was the same fact twice.
 *
 * Drag the sun to set the hour.
 */

/*
  Sizes inside this file are user units of the viewBox, not screen pixels, so
  they stay fixed where the rest of the panel's type is fluid: the whole dial
  scales as one drawing, and a label that grew with the window while the ring
  around it did not would simply come loose.
*/
const SIZE = 360;
const C = SIZE / 2;

const R = 130;
const W = 42;
const INNER = R - W / 2;
const OUTER = R + W / 2;

/** Degrees clockwise from the top, with noon at the top. */
function angle(hour: number): number {
  return (((hour - 12) / 24) * 360 + 360) % 360;
}

function polar(r: number, deg: number) {
  const a = ((deg - 90) * Math.PI) / 180;
  return { x: C + r * Math.cos(a), y: C + r * Math.sin(a) };
}

/** An arc drawn as a stroked path, clockwise from one hour to another. */
function arc(r: number, fromHour: number, toHour: number): string {
  const start = angle(fromHour);
  let sweep = ((toHour - fromHour) / 24) * 360;
  // A full circle cannot be expressed as a single arc, so nudge it shut.
  if (sweep >= 360) sweep = 359.99;
  const a = polar(r, start);
  const b = polar(r, start + sweep);
  return `M ${a.x} ${a.y} A ${r} ${r} 0 ${sweep > 180 ? 1 : 0} 1 ${b.x} ${b.y}`;
}

interface Props {
  day: number;
  hour: number;
  minute: number;
  /** How many of the 24 hours are lit, from the world's own setting. */
  daylightHours: number;
  dawnHour: number;
  /** The in-game hour the horde arrives, marked when it lands tonight. */
  hordeHour?: number;
  hordeTonight?: boolean;
  onSetTime: (hour: number, minute: number) => void;
}

/**
 * The nearest five minutes.
 *
 * Landing on the whole hour meant a drag to quarter to five committed as four
 * o'clock — the sun visibly jumped backwards out from under the pointer on
 * release, which is the one thing a dial must never do. Five minutes is a
 * quarter of a degree of arc: fine enough to feel exact, coarse enough that
 * the number you let go on is the number you get.
 */
function snap(hour: number): number {
  return Math.round(hour * 12) / 12;
}

export function DayDial({
  day,
  hour,
  minute,
  daylightHours,
  dawnHour,
  hordeHour,
  hordeTonight,
  onSetTime,
}: Props) {
  const svgRef = useRef<SVGSVGElement>(null);
  // While dragging, the sun follows the pointer and the server is only told
  // once the pointer lifts. Sending a command per pixel would be absurd.
  const [dragHour, setDragHour] = useState<number | null>(null);

  const now = hour + minute / 60;
  // hourAt counts round from noon, so the bottom-left quadrant comes back as
  // 24 to 30 rather than 0 to 6. Wrap it before anything compares it to dawn.
  const shown = (dragHour !== null ? snap(dragHour) : now) % 24;
  const duskHour = dawnHour + daylightHours;
  const lit = shown >= dawnHour && shown < duskHour;

  /** Where the pointer is, in the dial's own terms. */
  function pointerAt(event: React.PointerEvent) {
    const svg = svgRef.current;
    if (!svg) return null;
    const box = svg.getBoundingClientRect();
    const x = ((event.clientX - box.left) / box.width) * SIZE - C;
    const y = ((event.clientY - box.top) / box.height) * SIZE - C;
    const deg = ((Math.atan2(y, x) * 180) / Math.PI + 90 + 360) % 360;
    // Counted round from noon, so this runs 12 to 36 and wants wrapping before
    // it is compared to anything.
    return { hour: (deg / 360) * 24 + 12, distance: Math.hypot(x, y) };
  }

  /**
   * Whether a press at this point should take hold of the sun.
   *
   * Only the band is live. The hole in the middle holds the readout, and the
   * corners of the square are empty to look at — but they still have an angle,
   * so without this a click on blank space set the world clock.
   *
   * Deliberately not applied once a drag is under way: an arm swings in an arc
   * wider than a 42px band, and testing the radius on every move meant the sun
   * stuck the moment the pointer drifted off the ring.
   */
  function onBand(distance: number): boolean {
    return distance >= INNER - 10 && distance <= OUTER + 14;
  }

  /*
    The sky is built from half-hour segments rather than two flat arcs, so the
    light can actually rise and fall across the band: brightest at noon,
    guttering at dawn and dusk. Two arcs would have drawn daylight as a single
    slab, which is not what daylight looks like and not how it is played.
  */
  const step = 0.5;
  const segments = Array.from({ length: 24 / step }, (_, i) => {
    const from = i * step;
    const daytime = from >= dawnHour && from < duskHour;
    // Height of the sun, 0 at the horizons and 1 at midday.
    const elevation = daytime ? Math.sin((Math.PI * (from - dawnHour)) / daylightHours) : 0;
    // Depth of the night, mirrored: darkest in the small hours.
    const nightDepth = daytime
      ? 0
      : Math.sin(
          (Math.PI * ((from < dawnHour ? from + 24 : from) - duskHour)) / (24 - daylightHours),
        );
    // Mixed into the background rather than laid over it at low alpha: the
    // segments have to overlap to hide the seam between them, and two
    // translucent strokes on top of each other are darker than one, which drew
    // the sky as stripes. Opaque colours overlap invisibly.
    const tint = daytime
      ? `var(--bone) ${(5 + 25 * elevation).toFixed(1)}%`
      : `${hordeTonight ? "var(--crimson-lit)" : "var(--crimson)"} ${(8 + 34 * nightDepth).toFixed(1)}%`;
    return {
      from,
      d: arc(R, from, from + step * 2),
      color: `color-mix(in oklab, var(--ash-950), ${tint})`,
    };
  });

  const sun = polar(R, angle(shown));

  return (
    <div className="relative w-full select-none">
      <svg
        ref={svgRef}
        viewBox={`0 0 ${SIZE} ${SIZE}`}
        className="w-full touch-none"
        role="group"
        aria-label={`Day ${day}, ${String(hour).padStart(2, "0")}:${String(minute).padStart(2, "0")}`}
        onPointerDown={(e) => {
          const at = pointerAt(e);
          if (!at || !onBand(at.distance)) return;
          e.currentTarget.setPointerCapture(e.pointerId);
          setDragHour(at.hour);
        }}
        onPointerMove={(e) => {
          if (dragHour === null) return;
          const at = pointerAt(e);
          if (at) setDragHour(at.hour);
        }}
        onPointerUp={() => {
          if (dragHour !== null) {
            const h = snap(dragHour) % 24;
            onSetTime(Math.floor(h), Math.round((h % 1) * 60));
          }
          setDragHour(null);
        }}
        onPointerCancel={() => setDragHour(null)}
      >
        <path d={arc(R, 0, 24)} strokeWidth={W} fill="none" className="stroke-ash-950" />

        {segments.map((seg) => (
          <path key={seg.from} d={seg.d} strokeWidth={W} fill="none" stroke={seg.color} />
        ))}

        {/* The hour the horde arrives, on the night it arrives. */}
        {hordeTonight && hordeHour !== undefined && (
          <path
            d={arc(R, hordeHour, hordeHour + 0.3)}
            strokeWidth={W}
            fill="none"
            className="animate-breathe stroke-crimson-lit"
          />
        )}

        {/* The two horizons, cut clean through the band and named outside it. */}
        {[
          { hour: dawnHour, label: "dawn" },
          { hour: duskHour % 24, label: "dusk" },
        ].map((edge) => {
          const deg = angle(edge.hour);
          const a = polar(INNER, deg);
          const b = polar(OUTER, deg);
          const text = polar(INNER - 16, deg);
          return (
            <g key={edge.label} className="pointer-events-none">
              <line
                x1={a.x}
                y1={a.y}
                x2={b.x}
                y2={b.y}
                strokeWidth={1}
                className="stroke-bone/50"
              />
              <text
                x={text.x}
                y={text.y}
                dy="3"
                textAnchor="middle"
                className="fill-bone-dim text-[9px] tracking-[0.18em] uppercase"
                style={{ fontFamily: "var(--font-display)" }}
              >
                {edge.label}
              </text>
            </g>
          );
        })}

        {Array.from({ length: 24 }, (_, h) => {
          const major = h % 6 === 0;
          const deg = angle(h);
          const a = polar(INNER, deg);
          const b = polar(INNER + (major ? 7 : 4), deg);
          return (
            <line
              key={h}
              x1={a.x}
              y1={a.y}
              x2={b.x}
              y2={b.y}
              strokeWidth={1}
              className={major ? "stroke-bone/40" : "stroke-bone/10"}
            />
          );
        })}

        {/* Numerals outside the band, leaving the middle clear for the readout. */}
        {[0, 6, 12, 18].map((h) => {
          const p = polar(OUTER + 17, angle(h));
          return (
            <text
              key={h}
              x={p.x}
              y={p.y}
              dy="3.5"
              textAnchor="middle"
              className="pointer-events-none fill-bone-dim text-[10px]"
              style={{ fontFamily: "var(--font-mono)" }}
            >
              {String(h).padStart(2, "0")}
            </text>
          );
        })}

        {/*
          The sun rides the band, and is the handle. In daylight it is a filled
          disc with a halo; after dusk it becomes a hollow ring — the moon —
          which goes crimson on the night that matters.
        */}
        <g
          style={{
            filter: `drop-shadow(0 0 ${lit ? 10 : 7}px ${lit ? "var(--ember)" : "var(--crimson-lit)"})`,
          }}
        >
          <circle cx={sun.x} cy={sun.y} r={11} className="fill-background" />
          <circle
            cx={sun.x}
            cy={sun.y}
            r={8}
            strokeWidth={lit ? 0 : 2.5}
            className={cn(
              "cursor-grab",
              lit
                ? "fill-ember"
                : cn("fill-none", hordeTonight ? "stroke-crimson-lit" : "stroke-bone-dim"),
            )}
          />
        </g>
      </svg>

      {/* The readout lives in the hole in the middle. */}
      <div className="pointer-events-none absolute inset-0 flex flex-col items-center justify-center">
        <span className="stencil">Day</span>
        <span className="figure text-5xl leading-none">{day}</span>
        <span className="readout mt-1.5 text-base text-bone-dim">
          {`${String(Math.floor(shown)).padStart(2, "0")}:${String(Math.floor((shown % 1) * 60)).padStart(2, "0")}`}
        </span>
      </div>
    </div>
  );
}

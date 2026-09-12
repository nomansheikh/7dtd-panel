import { useRef, useState } from "react";
import { cn } from "@/lib/utils";

/**
 * One weather parameter, set by dragging it.
 *
 * Each of these used to be a slider with its own Apply button beside it, which
 * is six buttons to say one thing — the same fault the clock had when moving
 * the time took a field and a Go. Here the bar is the control: drag it, let go,
 * and that is the command. Nothing to press afterwards.
 *
 * The bar shows what the biome is actually doing, not what was last typed. The
 * server reports a cleared override and one deliberately set to zero
 * identically, so the last-typed number is not a fact about the world; the
 * biome's own reading is.
 */

interface Props {
  label: string;
  /** What the selected biome currently reports, in the parameter's own units. */
  value: number;
  min: number;
  max: number;
  /** Turns a value into the word a person would use for it. */
  describe: (value: number) => string;
  onCommit: (value: number) => void;
}

export function WeatherMeter({ label, value, min, max, describe, onCommit }: Props) {
  const trackRef = useRef<HTMLDivElement>(null);
  const [dragging, setDragging] = useState<number | null>(null);

  const shown = dragging ?? value;
  const fraction = Math.min(1, Math.max(0, (shown - min) / (max - min)));

  function valueAt(event: React.PointerEvent): number {
    const track = trackRef.current;
    if (!track) return shown;
    const box = track.getBoundingClientRect();
    const t = Math.min(1, Math.max(0, (event.clientX - box.left) / box.width));
    return min + t * (max - min);
  }

  return (
    <div className="group/meter">
      <div className="flex items-baseline justify-between gap-3">
        <span className="stencil">{label}</span>
        <span
          className={cn(
            "readout text-xs transition-colors",
            dragging === null ? "text-bone-dim" : "text-bone",
          )}
        >
          {describe(shown)}
        </span>
      </div>

      {/*
        A tall, thin hit area around a short bar: the bar wants to be quiet in
        a column of six, but a 4px drag target is a joke on a laptop trackpad.
      */}
      <div
        ref={trackRef}
        role="slider"
        tabIndex={0}
        aria-label={label}
        aria-valuemin={min}
        aria-valuemax={max}
        aria-valuenow={Math.round(shown * 100) / 100}
        aria-valuetext={describe(shown)}
        className="relative mt-2 flex h-5 cursor-ew-resize touch-none items-center"
        onPointerDown={(e) => {
          e.currentTarget.setPointerCapture(e.pointerId);
          setDragging(valueAt(e));
        }}
        onPointerMove={(e) => {
          if (dragging === null) return;
          setDragging(valueAt(e));
        }}
        onPointerUp={() => {
          if (dragging !== null) onCommit(dragging);
          setDragging(null);
        }}
        onPointerCancel={() => setDragging(null)}
        onKeyDown={(e) => {
          const step = (max - min) / 20;
          if (e.key === "ArrowLeft") onCommit(Math.max(min, value - step));
          else if (e.key === "ArrowRight") onCommit(Math.min(max, value + step));
          else return;
          e.preventDefault();
        }}
      >
        <div className="h-1 w-full bg-bone/12">
          <div
            className={cn(
              "h-full transition-[width,background-color]",
              dragging === null ? "bg-bone/45 duration-500" : "bg-bone duration-0",
            )}
            style={{ width: `${fraction * 100}%` }}
          />
        </div>
        <span
          aria-hidden
          className={cn(
            "pointer-events-none absolute size-2.5 -translate-x-1/2 transition-[left,background-color]",
            dragging === null
              ? "bg-bone-dim duration-500 group-hover/meter:bg-bone"
              : "bg-bone shadow-[0_0_8px_var(--bone)] duration-0",
          )}
          style={{ left: `${fraction * 100}%` }}
        />
      </div>
    </div>
  );
}

/**
 * The three fixed layers that sit over the whole panel: grain, vignette and
 * the crimson bleed.
 *
 * None of them are decoration for its own sake. The bleed is driven by --moon,
 * which the app sets from the live game clock, so how red the edges of the
 * screen are is a readout of how close the horde is. Someone who uses this
 * panel for a week will know what day it is before their eyes reach a number.
 *
 * All three are inert: fixed, aria-hidden and pointer-events-none, so nothing
 * here can catch a click or reach a screen reader.
 */

/**
 * Fractal noise, rendered by the browser rather than shipped as an image. At
 * this frequency it reads as film grain over the near-black rather than as
 * texture, and it costs no bytes.
 */
const GRAIN =
  "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='160' height='160'%3E" +
  "%3Cfilter id='n'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='0.82' numOctaves='3' " +
  "stitchTiles='stitch'/%3E%3C/filter%3E%3Crect width='160' height='160' filter='url(%23n)'/%3E%3C/svg%3E";

export function Atmosphere() {
  return (
    <div aria-hidden className="pointer-events-none fixed inset-0 z-50 overflow-hidden">
      {/* The horde, bleeding in from the top corners. Invisible on the first
          morning of a cycle and unmissable on the seventh night. */}
      <div
        className="absolute inset-0 transition-opacity duration-1000"
        style={{
          // The floor is almost nothing. On the morning after a horde the
          // panel should be ash: a permanent red cast would spend the signal
          // before the week has started.
          opacity: "calc(0.06 + var(--moon-heat) * 0.94)",
          background:
            // Kept to the corners. A wash across the middle would sit behind
            // every number on the page and there is no amount of red that is
            // worth an unreadable readout.
            "radial-gradient(42% 34% at -2% -10%, color-mix(in oklab, var(--crimson), transparent calc(72% - var(--moon-heat) * 34%)), transparent 70%)," +
            "radial-gradient(38% 32% at 102% -8%, color-mix(in oklab, var(--crimson-lit), transparent calc(74% - var(--moon-heat) * 32%)), transparent 68%)," +
            "radial-gradient(80% 40% at 50% 112%, color-mix(in oklab, var(--crimson-deep), transparent calc(72% - var(--moon-heat) * 28%)), transparent 72%)",
        }}
      />

      {/*
        On the night itself the bleed breathes, slowly, like something
        approaching rather than something alarming.

        Two elements, not one: the keyframes animate opacity, and an animation
        wins over an inline style, so a single element ignored --moon-night and
        glowed red all week. The gate has to sit outside the animation.
      */}
      <div
        className="absolute inset-0 transition-opacity duration-700"
        style={{ opacity: "var(--moon-night)" }}
      >
        <div
          className="animate-bleed absolute inset-0"
          style={{
            background:
              "radial-gradient(120% 80% at 50% -5%, color-mix(in oklab, var(--crimson), transparent 68%), transparent 74%)",
          }}
        />
      </div>

      {/* Vignette. Always present, and heavier as the cycle runs down, which
          is what makes the screen feel like it is closing in. */}
      <div
        className="absolute inset-0"
        style={{
          background:
            "radial-gradient(118% 102% at 50% 32%, transparent 40%, oklch(0 0 0 / calc(0.4 + var(--moon-heat) * 0.3)) 100%)",
        }}
      />

      <div
        className="absolute inset-0 opacity-[0.045] mix-blend-overlay"
        style={{ backgroundImage: `url("${GRAIN}")`, backgroundSize: "160px 160px" }}
      />
    </div>
  );
}

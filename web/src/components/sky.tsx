import { Suspense, lazy, useState } from "react";
import { SkyPanel, W, H } from "@/components/sky-panel";

/**
 * The sky for a biome, rendered if the browser can and drawn if it cannot.
 *
 * three.js is most of a megabyte and nothing else in the panel wants it, so it
 * is behind a dynamic import and only arrives when somebody opens the Weather
 * tab. Everything else in the app loads exactly as it did.
 *
 * The flat version is not a stub. A self-hosted panel gets opened over a LAN on
 * whatever is to hand, and a machine with no working WebGL — a headless box
 * with software rendering, a locked-down browser, a remote session — still has
 * to show the weather. It stays the fallback rather than an error.
 */

const SkyScene = lazy(() => import("@/components/sky-scene"));

/** Whether this browser can actually give us a context, asked once. */
let webglMemo: boolean | null = null;
function hasWebGL(): boolean {
  if (webglMemo !== null) return webglMemo;
  try {
    const canvas = document.createElement("canvas");
    webglMemo = !!(canvas.getContext("webgl2") ?? canvas.getContext("webgl"));
  } catch {
    webglMemo = false;
  }
  return webglMemo;
}

/**
 * What the sky looks like, in one set of units.
 *
 * Everything except temperature is nought to one. The server reports each
 * quantity in its own scale and the commands take different ones again, so the
 * conversion happens once, where the two sources are reconciled, rather than
 * being re-guessed by every renderer.
 */
export interface SkyConditions {
  biome: string;
  state: string;
  cloud: number;
  rain: number;
  snow: number;
  fog: number;
  wind: number;
  /** Degrees, as the server reports them. */
  temperature: number;
}

interface Props {
  conditions: SkyConditions;
  hour: number;
  dawnHour: number;
  daylightHours: number;
  hordeTonight?: boolean;
}

export function Sky(props: Props) {
  // A lost context or a failed chunk drops back to the drawing rather than
  // taking the tab down with it.
  const [broken, setBroken] = useState(false);
  const flat = broken || !hasWebGL();

  return (
    <div className="dark relative w-full overflow-hidden" style={{ aspectRatio: `${W} / ${H}` }}>
      {flat ? (
        <SkyPanel {...props} className="absolute inset-0" />
      ) : (
        <ErrorBoundary onError={() => setBroken(true)}>
          <Suspense fallback={<SkyPanel {...props} className="absolute inset-0" />}>
            {/*
              A light grade, not a heavy one.

              A physically correct sky dropped raw into a panel drawn entirely
              in ash and bone reads as a window cut into a different
              application. A touch of warmth and a little off the saturation is
              enough to settle it into the page — and no more than that, because
              the scene has to stay a place you would want to look at rather
              than a muddy one.
            */}
            <div className="absolute inset-0 [filter:saturate(0.78)_contrast(1.06)_sepia(0.08)]">
              <SkyScene {...props} />
            </div>
            {/* Ties the top and bottom edges back into the panel behind them,
                so the frame sits in the page rather than on it. */}
            <div className="pointer-events-none absolute inset-0 bg-gradient-to-b from-ash-950/55 via-transparent to-ash-950/25" />
            {/* A vignette. The cheapest depth cue there is: the eye reads a
                darkened edge as the frame falling away from the subject. */}
            <div className="pointer-events-none absolute inset-0 [background:radial-gradient(ellipse_at_50%_55%,transparent_52%,var(--ash-950)_140%)] opacity-55" />
          </Suspense>
        </ErrorBoundary>
      )}
    </div>
  );
}

/**
 * Catches a render failure from the scene.
 *
 * Written out rather than pulled in: it is nine lines, it is the only place in
 * the panel that needs one, and a dependency for this would be silly.
 */
import { Component, type ErrorInfo, type ReactNode } from "react";

class ErrorBoundary extends Component<
  { children: ReactNode; onError: () => void },
  { failed: boolean }
> {
  state = { failed: false };

  static getDerivedStateFromError() {
    return { failed: true };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error("The rendered sky failed; falling back to the drawn one.", error, info);
    this.props.onError();
  }

  render() {
    return this.state.failed ? null : this.props.children;
  }
}

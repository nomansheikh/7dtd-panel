/**
 * What the map says when the game server is not drawing one.
 *
 * The renderer has to be switched on in serverconfig.xml before the server
 * starts. No console command and no preference change will start it afterwards:
 * setting EnableMapRendering at runtime changes what the server reports while
 * leaving the renderer switched off, so the panel would be lying if it said
 * the map was on its way. Say exactly what to do instead.
 */
export function MapUnavailable() {
  return (
    <div className="flex h-full items-center justify-center p-6">
      <div className="region max-w-xl">
        <div className="region-head">
          <span className="stencil">No map is being drawn</span>
        </div>
        <div className="space-y-3 p-4 text-sm text-bone-dim">
          <p>
            This server has map rendering switched off, so there are no tiles for the panel to show.
            It cannot be turned on from here — the game only starts its renderer at startup.
          </p>
          <p>
            To switch it on, edit the server's <code className="readout">serverconfig.xml</code>:
          </p>
          <pre className="readout overflow-x-auto border border-border bg-background p-3 text-2xs">
            {`<property name="EnableMapRendering" value="true"/>`}
          </pre>
          <p>
            Then restart the server. The map fills in as people explore: the game draws a square
            once somebody has loaded the ground under it, so a brand new world starts blank and
            grows.
          </p>
          <p className="text-2xs text-bone-faint">
            Ground explored before rendering was switched on stays blank until it is visited again,
            or until an admin runs <code className="readout">visitmap</code> in the console to walk
            the server over it.
          </p>
        </div>
      </div>
    </div>
  );
}

/** Shown while the map is on but nothing has been drawn yet. */
export function MapEmpty() {
  return (
    <div className="pointer-events-none absolute inset-0 z-[500] flex items-center justify-center">
      <div className="region pointer-events-auto max-w-md">
        <div className="region-head">
          <span className="stencil">Nothing drawn yet</span>
        </div>
        <div className="space-y-2 p-4 text-sm text-bone-dim">
          <p>
            Rendering is on, but the server has not drawn any ground yet. Squares appear as people
            explore.
          </p>
          <p className="text-2xs text-bone-faint">
            To fill in ground people have already been over, run{" "}
            <code className="readout">visitmap full</code> from the console. It walks the server
            across the whole world, which takes a few minutes and some CPU.
          </p>
        </div>
      </div>
    </div>
  );
}

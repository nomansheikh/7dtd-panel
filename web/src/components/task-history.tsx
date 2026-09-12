import { ago } from "@/lib/task-words";
import { type TaskRun } from "@/lib/api";

/**
 * What the panel has actually done.
 *
 * The point of a nightly restart is that nobody watches it happen, which makes
 * "did it?" the question this page has to answer without being asked.
 */
export function TaskHistory({ runs }: { runs: TaskRun[] }) {
  return (
    <aside className="shrink-0 border-b border-border lg:w-80 lg:border-b-0 lg:border-l xl:w-96">
      <div className="region-head">
        <span className="stencil">Lately</span>
        <span className="readout text-xs text-bone-faint">{runs.length}</span>
      </div>
      {runs.length === 0 ? (
        <p className="p-4 text-xs text-bone-faint md:px-6 lg:px-4">
          Nothing has run yet. Anything that does is listed here, whether it worked or not.
        </p>
      ) : (
        <ul className="divide-y divide-border/60">
          {runs.map((run, i) => (
            <li key={i} className="px-4 py-2 md:px-6 lg:px-4">
              <div className="flex items-baseline gap-2">
                <span className="min-w-0 flex-1 truncate text-xs text-bone-dim">{run.name}</span>
                <span className="readout text-2xs text-bone-faint">{ago(run.ranAt)}</span>
              </div>
              {run.error && <p className="mt-0.5 text-2xs text-destructive">{run.error}</p>}
            </li>
          ))}
        </ul>
      )}
    </aside>
  );
}

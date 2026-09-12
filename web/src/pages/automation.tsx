import { useState } from "react";
import { Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { TaskEditor } from "@/components/task-editor";
import { TaskHistory } from "@/components/task-history";
import { TaskRow } from "@/components/task-row";
import { useTasks } from "@/hooks/use-tasks";
import { type Task } from "@/lib/api";

/**
 * What the panel does when nobody is watching.
 *
 * The game server has no scheduler. It runs a world and answers questions about
 * it; anything that has to happen at a time, or because something happened,
 * needs somebody watching for it. The panel already is — it polls the clock and
 * holds the event stream — so this page is that somebody written down.
 *
 * Three of the four triggers are ordinary. The fourth is the reason the page
 * exists: the horde arrives on the game's clock, which runs at whatever rate
 * this world is set to, so "half an hour before" is a moving wall-clock target.
 * No crontab can express it. The panel recomputes it from the server's own day
 * length every time it looks.
 */

export function AutomationPage() {
  const { data, isLoading, error } = useTasks();
  const [editing, setEditing] = useState<Task | null | undefined>(undefined);

  if (isLoading) {
    return (
      <div className="space-y-px p-4">
        {Array.from({ length: 4 }, (_, i) => (
          <Skeleton key={i} className="h-16 w-full rounded-none" />
        ))}
      </div>
    );
  }

  if (error || !data) {
    return (
      <p className="p-8 text-center text-sm text-destructive">
        {error?.message ?? "The tasks could not be loaded."}
      </p>
    );
  }

  const on = data.tasks.filter((t) => t.enabled);

  return (
    <div className="min-h-0 flex-1 overflow-y-auto">
      <div className="flex flex-col lg:flex-row">
        <section className="min-w-0 flex-1">
          <div className="region-head">
            <span className="stencil">Tasks</span>
            <span className="readout text-xs text-bone-faint">
              {on.length} of {data.tasks.length} on
            </span>
            <Button
              variant="ghost"
              size="sm"
              className="ml-auto h-6 gap-1 px-2 text-xs"
              onClick={() => setEditing(null)}
            >
              <Plus className="size-3" />
              New task
            </Button>
          </div>

          {data.tasks.length === 0 ? (
            <Empty onNew={() => setEditing(null)} />
          ) : (
            <ul>
              {data.tasks.map((task) => (
                <TaskRow
                  key={task.name}
                  task={task}
                  allowDestructive={data.allowDestructive}
                  onEdit={() => setEditing(task)}
                />
              ))}
            </ul>
          )}
        </section>

        <TaskHistory runs={data.runs} />
      </div>

      {editing !== undefined && (
        <TaskEditor editing={editing ?? undefined} onClose={() => setEditing(undefined)} />
      )}
    </div>
  );
}

function Empty({ onNew }: { onNew: () => void }) {
  return (
    <div className="px-4 py-8 md:px-6">
      <p className="max-w-prose text-sm text-bone-dim">
        Nothing runs on its own yet. The game server has no scheduler, so this is where a nightly
        restart, a periodic save, or a warning before the horde arrives would live.
      </p>
      <ul className="mt-3 space-y-1 text-xs text-bone-faint">
        <li>
          <span className="text-bone-dim">Every day at 05:00</span> — warn, save, shut down
        </li>
        <li>
          <span className="text-bone-dim">30 minutes before the blood moon</span> — tell everybody
        </li>
        <li>
          <span className="text-bone-dim">When the last player leaves</span> — save the world
        </li>
        <li>
          <span className="text-bone-dim">Once the server has been up 6 hours</span> — restart it
        </li>
        <li>
          <span className="text-bone-dim">When a player joins</span> — hand them a starter kit
        </li>
      </ul>
      <Button variant="outline" size="sm" className="mt-4 gap-1" onClick={onNew}>
        <Plus className="size-3" />
        Write one
      </Button>
    </div>
  );
}

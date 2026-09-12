import { useState } from "react";
import { toast } from "sonner";
import { Pencil, Plus, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { Skeleton } from "@/components/ui/skeleton";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { TaskEditor } from "@/components/task-editor";
import { useDeleteTask, useSaveTask, useTasks } from "@/hooks/use-tasks";
import { type Task, type TaskRun } from "@/lib/api";
import { cn } from "@/lib/utils";

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

/** When it runs, in words. */
function when(task: Task): string {
  switch (task.trigger) {
    case "daily":
      return `Every day at ${task.at}`;
    case "every":
      return `Every ${humanMinutes(task.minutes ?? 0)}`;
    case "bloodmoon":
      return `${humanMinutes(task.minutes ?? 0)} before the blood moon`;
    case "join":
      return "When a player joins";
  }
}

function humanMinutes(m: number): string {
  if (m < 60) return `${m} minutes`;
  const hours = m / 60;
  return hours === 1 ? "1 hour" : `${Number.isInteger(hours) ? hours : hours.toFixed(1)} hours`;
}

/** How long ago, roughly, for a line that is read at a glance. */
function ago(iso: string): string {
  const seconds = Math.max(0, (Date.now() - new Date(iso).getTime()) / 1000);
  if (seconds < 90) return "just now";
  if (seconds < 3600) return `${Math.round(seconds / 60)}m ago`;
  if (seconds < 86400) return `${Math.round(seconds / 3600)}h ago`;
  return `${Math.round(seconds / 86400)}d ago`;
}

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
                <TaskRowItem
                  key={task.name}
                  task={task}
                  allowDestructive={data.allowDestructive}
                  onEdit={() => setEditing(task)}
                />
              ))}
            </ul>
          )}
        </section>

        <History runs={data.runs} />
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

function TaskRowItem({
  task,
  allowDestructive,
  onEdit,
}: {
  task: Task;
  allowDestructive: boolean;
  onEdit: () => void;
}) {
  const save = useSaveTask();
  const remove = useDeleteTask();
  const [confirming, setConfirming] = useState(false);

  const toggle = (enabled: boolean) =>
    save.mutate(
      {
        name: task.name,
        enabled,
        description: task.description,
        trigger: task.trigger,
        minutes: task.minutes,
        at: task.at,
        commands: task.commands.map((c) => c.line),
      },
      { onError: (err) => toast.error("That did not save", { description: err.message }) },
    );

  const blocked = task.commands.some((l) => l.blocked);

  return (
    <li
      className={cn(
        "group border-b border-border px-4 py-2.5 md:px-6",
        task.enabled ? "bg-accent/20" : "text-bone-faint",
      )}
    >
      <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
        <span className={cn("text-sm", task.enabled ? "text-bone" : "text-bone-dim")}>
          {task.name}
        </span>
        <span className="readout text-xs text-bone-faint">{when(task)}</span>
        {task.tier === "destructive" && (
          <span className="stencil text-crimson-lit">destructive</span>
        )}
        {task.lastRunAt && (
          <span className="readout text-2xs text-bone-faint">ran {ago(task.lastRunAt)}</span>
        )}

        <div className="ml-auto flex items-center gap-2">
          <span className="flex items-center gap-0.5 opacity-0 transition-opacity group-focus-within:opacity-100 group-hover:opacity-100">
            <Button
              variant="ghost"
              size="icon"
              className="size-6 text-bone-faint"
              aria-label={`Edit ${task.name}`}
              onClick={onEdit}
            >
              <Pencil className="size-3" />
            </Button>
            <Button
              variant="ghost"
              size="icon"
              className="size-6 text-bone-faint hover:text-crimson-lit"
              aria-label={`Delete ${task.name}`}
              onClick={() => setConfirming(true)}
            >
              <Trash2 className="size-3" />
            </Button>
          </span>
          <Switch checked={task.enabled} onCheckedChange={toggle} aria-label={`Run ${task.name}`} />
        </div>
      </div>

      {task.description && (
        <p className="mt-0.5 max-w-prose text-xs text-bone-faint">{task.description}</p>
      )}

      {/* What it will actually run, so the switch beside it is an informed one. */}
      <ul className="mt-1.5 space-y-0.5">
        {task.commands.map((line, i) => (
          <li key={i} className="flex flex-wrap items-baseline gap-2">
            <code className="readout text-2xs text-bone-dim">{line.line}</code>
            {line.tier !== "normal" && (
              <span
                className={cn(
                  "stencil",
                  line.tier === "destructive" ? "text-crimson-lit" : "text-ember",
                )}
              >
                {line.tier}
              </span>
            )}
            {line.problem && <span className="text-2xs text-destructive">{line.problem}</span>}
          </li>
        ))}
      </ul>

      {blocked && !allowDestructive && (
        <p className="mt-1 text-2xs text-bone-faint">
          A line here is destructive, and this panel was started with
          <code className="readout"> PANEL_ALLOW_DESTRUCTIVE=false</code>. It will not run until
          that is changed.
        </p>
      )}

      <AlertDialog open={confirming} onOpenChange={setConfirming}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete {task.name}?</AlertDialogTitle>
            <AlertDialogDescription>
              It stops running, and what it has done is forgotten with it.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Keep it</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => {
                remove.mutate(task.name, {
                  onSuccess: () => toast.success(`${task.name} is gone`),
                  onError: (err) => toast.error("Not deleted", { description: err.message }),
                });
                setConfirming(false);
              }}
            >
              Delete it
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </li>
  );
}

/**
 * What the panel has actually done.
 *
 * The point of a nightly restart is that nobody watches it happen, which makes
 * "did it?" the question this page has to answer without being asked.
 */
function History({ runs }: { runs: TaskRun[] }) {
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

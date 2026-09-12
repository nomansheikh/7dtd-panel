import { useState } from "react";
import { toast } from "sonner";
import { Pencil, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
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
import { useDeleteTask, useSaveTask } from "@/hooks/use-tasks";
import { ago, when } from "@/lib/task-words";
import { type Task } from "@/lib/api";
import { cn } from "@/lib/utils";

/**
 * One task: when it runs, what it will run, and the switch.
 *
 * The lines it will send are listed under it with their tier, because the
 * switch beside them is the moment somebody decides to let this happen
 * unattended, and that is the only moment the information is worth anything.
 */
export function TaskRow({
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

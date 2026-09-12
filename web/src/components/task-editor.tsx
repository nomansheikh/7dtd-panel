import { useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { CommandLines } from "@/components/command-lines";
import { Field } from "@/components/field";
import { TriggerPicker, triggerNeeds } from "@/components/trigger-picker";
import { useSaveTask } from "@/hooks/use-tasks";
import { INTERVALS, UPTIMES, WARNINGS } from "@/lib/triggers";
import { type Task, type TaskTrigger } from "@/lib/api";

/**
 * Writing something the panel will do on its own.
 *
 * The same shape as a chat command — a name, console lines, a switch — because
 * it is the same act with a different thing setting it off. What differs is the
 * trigger, and only one of the four is interesting: "before the blood moon"
 * moves with the game's clock, so the panel works out the wall-clock moment
 * from the server's own day length every time it checks.
 */

interface Props {
  editing?: Task;
  onClose: () => void;
}

export function TaskEditor({ editing, onClose }: Props) {
  const save = useSaveTask();

  const [name, setName] = useState(editing?.name ?? "");
  const [description, setDescription] = useState(editing?.description ?? "");
  const [trigger, setTrigger] = useState<TaskTrigger>(editing?.trigger ?? "daily");
  const [at, setAt] = useState(editing?.at ?? "05:00");
  const [minutes, setMinutes] = useState(editing?.minutes ?? 30);
  const [lines, setLines] = useState<string[]>(editing?.commands.map((c) => c.line) ?? [""]);

  // Switching to a trigger whose choices do not include the current value would
  // leave the select showing nothing, so move to the nearest sensible default.
  const pickTrigger = (next: TaskTrigger) => {
    setTrigger(next);
    if (next === "uptime" && !UPTIMES.some((u) => u.minutes === minutes)) setMinutes(360);
    if (next === "every" && !INTERVALS.some((i) => i.minutes === minutes)) setMinutes(30);
    if (next === "bloodmoon" && !WARNINGS.some((w) => w.minutes === minutes)) setMinutes(30);
    if (next === "gametime" && at === "05:00") setAt("21:00");
  };

  const trimmed = lines.map((l) => l.trim()).filter((l) => l !== "");
  const canSave = name.trim() !== "" && trimmed.length > 0;
  const { needs, takesMinutes } = triggerNeeds(trigger);

  const submit = () =>
    save.mutate(
      {
        name: name.trim().toLowerCase(),
        // Written and switched on separately, so the lines and their tiers can
        // be read back before anything runs unattended.
        enabled: editing?.enabled ?? false,
        description: description.trim(),
        trigger,
        minutes: takesMinutes ? minutes : 0,
        at: needs === "time" ? at : "",
        commands: trimmed,
      },
      {
        onSuccess: () => {
          toast.success(editing ? `Saved ${name}` : `Wrote ${name}`, {
            description: editing ? undefined : "It is off until you switch it on.",
          });
          onClose();
        },
        onError: (err) => toast.error("Not saved", { description: err.message }),
      },
    );

  return (
    <Sheet open onOpenChange={(open) => !open && onClose()}>
      <SheetContent side="right" className="w-full gap-0 overflow-y-auto p-0 sm:max-w-xl">
        <SheetHeader className="region-head shrink-0 space-y-0 p-3 md:px-4">
          <SheetTitle className="stencil">
            {editing ? `Edit ${editing.name}` : "New task"}
          </SheetTitle>
          <SheetDescription className="sr-only">
            Give it a name, say when it runs, and list the commands.
          </SheetDescription>
        </SheetHeader>

        <div className="space-y-5 p-4 md:px-6">
          <Field label="Called" hint="For your own memory. It is never typed by anybody.">
            <Input
              value={name}
              onChange={(e) => setName(e.target.value.toLowerCase().replace(/\s+/g, "-"))}
              placeholder="nightly-restart"
              className="h-8 text-sm"
              disabled={Boolean(editing)}
              autoFocus={!editing}
            />
          </Field>

          <Field label="What it does" hint="Shown in the list beside it.">
            <Input
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Restart, with warnings"
              className="h-8 text-sm"
            />
          </Field>

          <TriggerPicker
            trigger={trigger}
            onTrigger={pickTrigger}
            at={at}
            onAt={setAt}
            minutes={minutes}
            onMinutes={setMinutes}
          />

          <Field
            label="It runs"
            hint="Console commands, in order. It stops at the first one that fails, so a restart that could not warn anybody does not go on to shut the server down."
          >
            <CommandLines
              lines={lines}
              onChange={setLines}
              placeholder={'say "restarting in 5 minutes"'}
            />
          </Field>

          {(trigger === "join" || trigger === "leave" || trigger === "death") && (
            <p className="text-2xs text-bone-faint">
              <code className="readout">{"{player}"}</code> and{" "}
              <code className="readout">{"{entityid}"}</code> stand for the player it ran for. A
              name is put in as plain characters only, so it cannot reshape the line.
            </p>
          )}
        </div>

        <div className="sticky bottom-0 flex shrink-0 gap-2 border-t border-border bg-background p-3 md:px-6">
          <Button className="flex-1" disabled={!canSave || save.isPending} onClick={submit}>
            {editing ? "Save changes" : "Write it"}
          </Button>
          <Button variant="ghost" onClick={onClose}>
            Cancel
          </Button>
        </div>
      </SheetContent>
    </Sheet>
  );
}

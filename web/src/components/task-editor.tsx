import { useState } from "react";
import { toast } from "sonner";
import { Plus, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { useSaveTask } from "@/hooks/use-tasks";
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

const TRIGGERS: { value: TaskTrigger; label: string; hint: string }[] = [
  {
    value: "daily",
    label: "Every day at a time",
    hint: "Wall-clock time, in the panel's timezone. A nightly restart lives here.",
  },
  {
    value: "every",
    label: "On a repeat",
    hint: "Every so many real minutes, counted from the last time it ran.",
  },
  {
    value: "bloodmoon",
    label: "Before the blood moon",
    hint: "Worked out from this world's own day length, so it lands at the right moment however fast the days run. Fires once per horde.",
  },
  {
    value: "join",
    label: "When a player joins",
    hint: "Runs for whoever connected. {player} and {entityid} stand for them.",
  },
];

const INTERVALS = [
  { minutes: 15, label: "15 minutes" },
  { minutes: 30, label: "30 minutes" },
  { minutes: 60, label: "1 hour" },
  { minutes: 180, label: "3 hours" },
  { minutes: 360, label: "6 hours" },
  { minutes: 720, label: "12 hours" },
];

const WARNINGS = [
  { minutes: 5, label: "5 minutes before" },
  { minutes: 10, label: "10 minutes before" },
  { minutes: 15, label: "15 minutes before" },
  { minutes: 30, label: "30 minutes before" },
  { minutes: 60, label: "an hour before" },
];

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

  const trimmed = lines.map((l) => l.trim()).filter((l) => l !== "");
  const canSave = name.trim() !== "" && trimmed.length > 0;
  const spec = TRIGGERS.find((t) => t.value === trigger);

  const submit = () =>
    save.mutate(
      {
        name: name.trim().toLowerCase(),
        // Written and switched on separately, so the lines and their tiers can
        // be read back before anything runs unattended.
        enabled: editing?.enabled ?? false,
        description: description.trim(),
        trigger,
        minutes: trigger === "every" || trigger === "bloodmoon" ? minutes : 0,
        at: trigger === "daily" ? at : "",
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

          <Field label="When" hint={spec?.hint ?? ""}>
            <div className="flex flex-wrap items-center gap-2">
              <Select value={trigger} onValueChange={(v) => setTrigger(v as TaskTrigger)}>
                <SelectTrigger
                  className="h-8 w-auto gap-1.5 px-2 text-xs data-[size=default]:h-8"
                  aria-label="What sets it off"
                >
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {TRIGGERS.map((t) => (
                    <SelectItem key={t.value} value={t.value} className="text-xs">
                      {t.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>

              {trigger === "daily" && (
                <Input
                  type="time"
                  value={at}
                  onChange={(e) => setAt(e.target.value)}
                  className="readout h-8 w-28 text-xs"
                  aria-label="Time of day"
                />
              )}

              {(trigger === "every" || trigger === "bloodmoon") && (
                <Select value={String(minutes)} onValueChange={(v) => setMinutes(Number(v))}>
                  <SelectTrigger
                    className="h-8 w-auto gap-1.5 px-2 text-xs data-[size=default]:h-8"
                    aria-label="How long"
                  >
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {(trigger === "every" ? INTERVALS : WARNINGS).map((o) => (
                      <SelectItem key={o.minutes} value={String(o.minutes)} className="text-xs">
                        {o.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              )}
            </div>
          </Field>

          <Field
            label="It runs"
            hint="Console commands, in order. It stops at the first one that fails, so a restart that could not warn anybody does not go on to shut the server down."
          >
            <div className="space-y-1.5">
              {lines.map((line, i) => (
                <div key={i} className="flex items-center gap-1">
                  <span className="readout w-4 shrink-0 text-2xs text-bone-faint">{i + 1}</span>
                  <Input
                    value={line}
                    onChange={(e) =>
                      setLines((c) => c.map((l, at) => (at === i ? e.target.value : l)))
                    }
                    placeholder={'say "restarting in 5 minutes"'}
                    className="readout h-8 flex-1 text-xs"
                  />
                  <Button
                    variant="ghost"
                    size="icon"
                    className="size-8 shrink-0 text-bone-faint"
                    aria-label={`Remove line ${i + 1}`}
                    onClick={() => setLines((c) => c.filter((_, at) => at !== i))}
                  >
                    <Trash2 className="size-3" />
                  </Button>
                </div>
              ))}
              <Button
                variant="outline"
                size="sm"
                className="h-7 gap-1 text-xs"
                onClick={() => setLines((c) => [...c, ""])}
              >
                <Plus className="size-3" />
                {lines.length === 0 ? "Add a command" : "Another"}
              </Button>
            </div>
          </Field>

          {trigger === "join" && (
            <p className="text-2xs text-bone-faint">
              <code className="readout">{"{player}"}</code> and{" "}
              <code className="readout">{"{entityid}"}</code> stand for whoever joined. A name is
              put in as plain characters only, so it cannot reshape the line.
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

function Field({
  label,
  hint,
  children,
}: {
  label: string;
  hint: string;
  children: React.ReactNode;
}) {
  return (
    <div className="space-y-1.5">
      <span className="stencil">{label}</span>
      {children}
      {hint && <p className="max-w-prose text-2xs text-bone-faint">{hint}</p>}
    </div>
  );
}

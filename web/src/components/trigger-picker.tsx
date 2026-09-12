import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Field } from "@/components/field";
import { INTERVALS, TRIGGERS, UPTIMES, WARNINGS } from "@/lib/triggers";
import { type TaskTrigger } from "@/lib/api";

/**
 * Choosing what sets a task off, and the one extra control that choice implies.
 *
 * Each trigger declares what it needs — a time, an interval, a warning, an
 * uptime — rather than the form checking for each kind by name, so adding a
 * trigger is a row in a table rather than another branch here.
 */
export function TriggerPicker({
  trigger,
  onTrigger,
  at,
  onAt,
  minutes,
  onMinutes,
}: {
  trigger: TaskTrigger;
  onTrigger: (next: TaskTrigger) => void;
  at: string;
  onAt: (next: string) => void;
  minutes: number;
  onMinutes: (next: number) => void;
}) {
  const spec = TRIGGERS.find((t) => t.value === trigger);
  const needs = spec?.needs;
  const choices =
    needs === "interval"
      ? INTERVALS
      : needs === "before"
        ? WARNINGS
        : needs === "uptime"
          ? UPTIMES
          : null;

  return (
    <Field label="When" hint={spec?.hint ?? ""}>
      <div className="flex flex-wrap items-center gap-2">
        <Select value={trigger} onValueChange={(v) => onTrigger(v as TaskTrigger)}>
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

        {needs === "time" && (
          <Input
            type="time"
            value={at}
            onChange={(e) => onAt(e.target.value)}
            className="readout h-8 w-28 text-xs"
            aria-label={trigger === "gametime" ? "Hour of the game's day" : "Time of day"}
          />
        )}

        {choices && (
          <Select value={String(minutes)} onValueChange={(v) => onMinutes(Number(v))}>
            <SelectTrigger
              className="h-8 w-auto gap-1.5 px-2 text-xs data-[size=default]:h-8"
              aria-label="How long"
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {choices.map((o) => (
                <SelectItem key={o.minutes} value={String(o.minutes)} className="text-xs">
                  {o.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        )}
      </div>
    </Field>
  );
}

/** Whether this trigger takes a minutes value, which decides what gets sent. */
export function triggerNeeds(trigger: TaskTrigger) {
  const spec = TRIGGERS.find((t) => t.value === trigger);
  return { needs: spec?.needs, takesMinutes: spec?.needs !== undefined && spec.needs !== "time" };
}

import { useState } from "react";
import { toast } from "sonner";
import { Plus, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { useSaveChatCommand } from "@/hooks/use-chat";
import { type ChatAudience, type ChatCommand, type ChatPlaceholder } from "@/lib/api";
import { cn } from "@/lib/utils";

/**
 * Writing a command of your own.
 *
 * There is no menu of approved actions here, and no list of commands the panel
 * is willing to run. Whoever installed this already has a console page that
 * runs anything they type; refusing them the same line behind a chat trigger
 * would be a rule that holds nowhere else in the product, enforced against the
 * owner of the machine.
 *
 * What the panel does instead is refuse to be vague about it. A new command
 * arrives switched off and set to admins, the tier of every line it runs is
 * shown in the list once saved, and turning it on is a separate, deliberate
 * act from writing it.
 */

interface Props {
  /** The command being changed, or undefined when writing a new one. */
  editing?: ChatCommand;
  prefix: string;
  placeholders: ChatPlaceholder[];
  onClose: () => void;
}

export function CommandEditor({ editing, prefix, placeholders, onClose }: Props) {
  const save = useSaveChatCommand();

  const [name, setName] = useState(editing?.name ?? "");
  const [description, setDescription] = useState(editing?.summary ?? "");
  const [reply, setReply] = useState(editing?.reply ?? "");
  const [lines, setLines] = useState<string[]>(editing?.commands?.map((c) => c.line) ?? []);

  const trimmedLines = lines.map((l) => l.trim()).filter((l) => l !== "");
  const canSave = name.trim() !== "" && (reply.trim() !== "" || trimmedLines.length > 0);

  const submit = () => {
    save.mutate(
      {
        name: name.trim().toLowerCase(),
        // A new command is written and switched on separately. Between those
        // two acts the panel shows what each of its lines actually is, which
        // is the only moment that information changes anybody's mind.
        enabled: editing?.enabled ?? false,
        audience: (editing?.audience ?? "admins") as ChatAudience,
        cooldownSeconds: editing?.cooldownSeconds ?? 0,
        description: description.trim(),
        reply: reply.trim(),
        commands: trimmedLines,
      },
      {
        onSuccess: () => {
          toast.success(editing ? `Saved ${prefix}${name}` : `Wrote ${prefix}${name}`, {
            description: editing
              ? undefined
              : "It is off until you switch it on, and set to admins only.",
          });
          onClose();
        },
        onError: (err) => toast.error("Not saved", { description: err.message }),
      },
    );
  };

  return (
    <Sheet open onOpenChange={(open) => !open && onClose()}>
      <SheetContent side="right" className="w-full gap-0 overflow-y-auto p-0 sm:max-w-xl">
        <SheetHeader className="region-head shrink-0 space-y-0 p-3 md:px-4">
          <SheetTitle className="stencil">
            {editing ? `Edit ${prefix}${editing.name}` : "Write a command"}
          </SheetTitle>
          <SheetDescription className="sr-only">
            Give it a name, something to say, and any console commands to run.
          </SheetDescription>
        </SheetHeader>

        <div className="space-y-5 p-4 md:px-6">
          <Field
            label="Typed as"
            hint="Lowercase letters, digits, dash and underscore. This is what a player types."
          >
            <div className="flex items-center gap-1">
              <span className="readout text-sm text-bone-faint">{prefix}</span>
              <Input
                value={name}
                onChange={(e) => setName(e.target.value.toLowerCase().replace(/\s+/g, ""))}
                placeholder="discord"
                className="h-8 flex-1 text-sm"
                disabled={Boolean(editing)}
                autoFocus={!editing}
              />
            </div>
          </Field>

          <Field label="What it does" hint="Shown here and in !help. For your own memory.">
            <Input
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Where we talk"
              className="h-8 text-sm"
            />
          </Field>

          <Field
            label="It says"
            hint="Sent back to whoever asked, and only to them. Leave it empty to act silently."
          >
            <Input
              value={reply}
              onChange={(e) => setReply(e.target.value)}
              placeholder="Join us: discord.gg/example"
              className="h-8 text-sm"
            />
          </Field>

          <Field
            label="It runs"
            hint="Console commands, in order. Anything the console page will run, it will run."
          >
            <div className="space-y-1.5">
              {lines.map((line, i) => (
                <div key={i} className="flex items-center gap-1">
                  <span className="readout w-4 shrink-0 text-2xs text-bone-faint">{i + 1}</span>
                  <Input
                    value={line}
                    onChange={(e) =>
                      setLines((current) => current.map((l, at) => (at === i ? e.target.value : l)))
                    }
                    placeholder="give {entityid} resourceWood 500"
                    className="readout h-8 flex-1 text-xs"
                  />
                  <Button
                    variant="ghost"
                    size="icon"
                    className="size-8 shrink-0 text-bone-faint"
                    aria-label={`Remove line ${i + 1}`}
                    onClick={() => setLines((current) => current.filter((_, at) => at !== i))}
                  >
                    <Trash2 className="size-3" />
                  </Button>
                </div>
              ))}
              <Button
                variant="outline"
                size="sm"
                className="h-7 gap-1 text-xs"
                onClick={() => setLines((current) => [...current, ""])}
              >
                <Plus className="size-3" />
                {lines.length === 0 ? "Add a command" : "Another"}
              </Button>
            </div>
          </Field>

          <div className="border-t border-border pt-4">
            <span className="stencil">Stands for</span>
            <dl className="mt-2 space-y-1">
              {placeholders.map((p) => (
                <div key={p.token} className="flex gap-2 text-2xs">
                  <dt className="readout w-24 shrink-0 text-bone-dim">{p.token}</dt>
                  <dd className="text-bone-faint">{p.means}</dd>
                </div>
              ))}
            </dl>
            <p className="mt-2 text-2xs text-bone-faint">
              What a player types is put in as plain words only. It cannot add a quote or a second
              command, so the line that runs is the line you wrote.
            </p>
          </div>
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
    <div className={cn("space-y-1.5")}>
      <span className="stencil">{label}</span>
      {children}
      <p className="text-2xs text-bone-faint">{hint}</p>
    </div>
  );
}

import { useState } from "react";
import { toast } from "sonner";
import { MessageSquare, Plus, Trash2 } from "lucide-react";
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
import { ItemPicker } from "@/components/item-picker";
import { useChatCommands, useDeleteKit, useKits, useSaveChatCommand } from "@/hooks/use-chat";
import { useServerId } from "@/hooks/use-servers";
import { type ChatAudience, type ChatCommand, type Kit } from "@/lib/api";
import { cn } from "@/lib/utils";

/**
 * The bot: what the panel will answer when a player types in chat.
 *
 * This is the one page that is not a view onto the game server. The server has
 * no idea any of this exists — it streams chat out of one endpoint and takes
 * commands in through another, and never joins the two. The panel is holding
 * both ends already, so it can: a player types !kit starter, and a run of give
 * commands goes back.
 *
 * Everything here is off until somebody turns it on, and the page is built to
 * make that decision rather than to hide it. Hence the transcript at the top:
 * the thing being configured happens somewhere the operator is not, and a list
 * of switches with no picture of the result is a list of guesses.
 */

/*
  Cooldowns as words.

  A number field asking for seconds is a small cruelty: 21600 is not a length
  of time anybody holds in their head. These are the lengths that get chosen,
  and anything else an operator has already set is shown alongside them rather
  than silently rounded.
*/
const COOLDOWNS = [
  { seconds: 0, label: "No limit" },
  { seconds: 10, label: "10 seconds" },
  { seconds: 30, label: "30 seconds" },
  { seconds: 300, label: "5 minutes" },
  { seconds: 3600, label: "1 hour" },
  { seconds: 21600, label: "6 hours" },
  { seconds: 86400, label: "Once a day" },
];

function humanCooldown(seconds: number): string {
  const known = COOLDOWNS.find((c) => c.seconds === seconds);
  if (known) return known.label;
  if (seconds < 60) return `${seconds} seconds`;
  if (seconds < 3600) return `${Math.round(seconds / 60)} minutes`;
  return `${Math.round(seconds / 360) / 10} hours`;
}

/** The example exchange each command is worth, for the transcript. */
const EXAMPLES: Record<string, { asked: string; answered: string }> = {
  help: { asked: "!help", answered: "You can use: !help, !day, !bloodmoon, !kit <name>" },
  day: { asked: "!day", answered: "Day 12, 14:30. Dark in 7h30m of game time (19 minutes real)." },
  bloodmoon: { asked: "!bloodmoon", answered: "Day 14, which is 2 days away." },
  players: { asked: "!players", answered: "3 players online: aria, nullish, ren." },
  kit: { asked: "!kit starter", answered: "The starter kit is at your feet — 501 items." },
};

export function ChatPage() {
  const { data, isLoading, error } = useChatCommands();
  const kits = useKits();
  const [building, setBuilding] = useState(false);

  if (isLoading) {
    return (
      <div className="space-y-px p-4">
        {Array.from({ length: 6 }, (_, i) => (
          <Skeleton key={i} className="h-20 w-full rounded-none" />
        ))}
      </div>
    );
  }

  if (error || !data) {
    return (
      <p className="p-8 text-center text-sm text-destructive">
        {error?.message ?? "The chat commands could not be loaded."}
      </p>
    );
  }

  const on = data.commands.filter((c) => c.enabled);

  return (
    <div className="min-h-0 flex-1 overflow-y-auto">
      <Transcript prefix={data.prefix} enabled={on} />

      <section>
        <div className="region-head">
          <span className="stencil">Commands</span>
          <span className="readout text-xs text-bone-faint">
            {on.length} of {data.commands.length} on
          </span>
        </div>
        <ul>
          {data.commands.map((command) => (
            <CommandRow key={command.name} command={command} kits={kits.data ?? []} />
          ))}
        </ul>
      </section>

      <section>
        <div className="region-head">
          <span className="stencil">Kits</span>
          <span className="readout text-xs text-bone-faint">{kits.data?.length ?? 0} saved</span>
          <Button
            variant="ghost"
            size="sm"
            className="ml-auto h-6 gap-1 px-2 text-xs"
            onClick={() => setBuilding(true)}
          >
            <Plus className="size-3" />
            New kit
          </Button>
        </div>
        <Kits kits={kits.data ?? []} loading={kits.isLoading} onBuild={() => setBuilding(true)} />
      </section>

      {building && <ItemPicker purpose="kit" onClose={() => setBuilding(false)} />}
    </div>
  );
}

/**
 * What a player sees, drawn as the game's own chat.
 *
 * The whole feature happens on somebody else's screen. Showing the result in
 * the shape it actually arrives in — a name, a line, an answer from the server
 * — is the difference between an operator knowing what they just switched on
 * and finding out from a player.
 */
function Transcript({ prefix, enabled }: { prefix: string; enabled: ChatCommand[] }) {
  const shown = enabled.slice(0, 3);

  return (
    <div className="border-b border-border px-4 py-5 md:px-6">
      <div className="flex items-start gap-4">
        <MessageSquare className="mt-0.5 hidden size-4 shrink-0 text-bone-faint sm:block" />
        <div className="min-w-0 flex-1">
          <p className="max-w-prose text-sm text-bone-dim">
            The panel watches chat and answers. A player types{" "}
            <code className="readout text-bone">{prefix}something</code> and gets a reply only they
            can see. Nothing is on until you turn it on, and nothing here can kick, ban or wipe
            anything.
          </p>

          {shown.length === 0 ? (
            <p className="mt-3 text-xs text-bone-faint">
              Nothing is switched on, so the panel stays silent in chat.
            </p>
          ) : (
            <div className="panel mt-4 max-w-xl space-y-1.5 p-3">
              {shown.map((command) => {
                const example = EXAMPLES[command.name];
                if (!example) return null;
                return (
                  <div key={command.name} className="space-y-0.5 text-xs">
                    <p className="readout">
                      <span className="text-bone-faint">&lt;</span>
                      <span className="text-ember">nullish</span>
                      <span className="text-bone-faint">&gt; </span>
                      <span className="text-bone">{example.asked}</span>
                    </p>
                    <p className="readout pl-3 text-bone-dim">
                      <span className="text-crimson-lit">[Server] </span>
                      {example.answered}
                    </p>
                  </div>
                );
              })}
              <p className="pt-1 text-2xs text-bone-faint">
                An example of the shape, not a live reading.
              </p>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

/**
 * One command.
 *
 * The controls appear only once it is on. A switched-off command showing an
 * audience and a cooldown invites the reading that the settings are doing
 * something, and they are not.
 */
function CommandRow({ command, kits }: { command: ChatCommand; kits: Kit[] }) {
  const save = useSaveChatCommand();

  const update = (change: Partial<ChatCommand>) => {
    const next = {
      name: command.name,
      enabled: command.enabled,
      audience: command.audience,
      cooldownSeconds: command.cooldownSeconds,
      ...change,
    };
    save.mutate(next, {
      onError: (err) => toast.error("That did not save", { description: err.message }),
    });
  };

  // The one command that hands out loot, open to everybody, with no kits to
  // hand out: worth saying, because it will look broken from in game.
  const noKits = command.name === "kit" && command.enabled && kits.length === 0;

  return (
    <li
      className={cn("border-b border-border px-4 py-3 md:px-6", command.enabled && "bg-accent/20")}
    >
      <div className="flex items-start gap-4">
        <div className="min-w-0 flex-1">
          <div className="flex items-baseline gap-2">
            <code className="readout text-sm text-bone">{command.usage}</code>
            {command.acts && (
              <span className="stencil text-ember">
                {command.audience === "everyone" && command.enabled ? "hands out loot" : "acts"}
              </span>
            )}
          </div>
          <p className="mt-0.5 max-w-prose text-xs text-bone-dim">{command.summary}</p>
        </div>

        <Switch
          checked={command.enabled}
          onCheckedChange={(enabled) => update({ enabled })}
          aria-label={`Answer ${command.usage}`}
        />
      </div>

      {command.enabled && (
        <div className="mt-3 space-y-2 border-l border-border pl-3">
          <Choice
            label="Who"
            options={[
              { value: "everyone", label: "Everyone" },
              { value: "admins", label: "Admins only" },
            ]}
            value={command.audience}
            onChange={(audience) => update({ audience: audience as ChatAudience })}
          />
          <Choice
            label="How often"
            options={cooldownOptions(command.cooldownSeconds)}
            value={String(command.cooldownSeconds)}
            onChange={(seconds) => update({ cooldownSeconds: Number(seconds) })}
          />
          {command.audience === "admins" && (
            <p className="text-2xs text-bone-faint">
              Admins are whoever is on the game's own admin list, not who can sign in here.
            </p>
          )}
          {noKits && (
            <p className="text-2xs text-ember">
              There are no kits yet, so this will tell anybody who asks that there are none.
            </p>
          )}
        </div>
      )}
    </li>
  );
}

/**
 * The preset lengths, plus whatever this command is already set to.
 *
 * A value that is not a preset — set by an earlier version, or by hand — goes
 * in its place along the ramp rather than on the end, so the row stays a scale
 * running short to long instead of a scale with an odd one appended.
 */
function cooldownOptions(current: number) {
  const all = COOLDOWNS.some((c) => c.seconds === current)
    ? COOLDOWNS
    : [...COOLDOWNS, { seconds: current, label: humanCooldown(current) }].sort(
        (a, b) => a.seconds - b.seconds,
      );
  return all.map((c) => ({ value: String(c.seconds), label: c.label }));
}

function Choice({
  label,
  options,
  value,
  onChange,
}: {
  label: string;
  options: { value: string; label: string }[];
  value: string;
  onChange: (value: string) => void;
}) {
  return (
    <div className="flex flex-wrap items-center gap-1">
      <span className="stencil mr-1 w-20 shrink-0">{label}</span>
      {options.map((option) => (
        <button
          key={option.value}
          type="button"
          onClick={() => onChange(option.value)}
          className={cn(
            "border px-2 py-0.5 text-xs transition-colors",
            option.value === value
              ? "border-border bg-accent text-bone"
              : "border-transparent text-bone-faint hover:text-bone-dim",
          )}
        >
          {option.label}
        </button>
      ))}
    </div>
  );
}

/* ------------------------------------------------------------------ kits -- */

function Kits({ kits, loading, onBuild }: { kits: Kit[]; loading: boolean; onBuild: () => void }) {
  const [editing, setEditing] = useState<Kit | null>(null);
  const [confirming, setConfirming] = useState<Kit | null>(null);
  const remove = useDeleteKit();

  if (loading) {
    return (
      <div className="grid gap-px p-4 sm:grid-cols-2 lg:grid-cols-3">
        {Array.from({ length: 3 }, (_, i) => (
          <Skeleton key={i} className="h-24 rounded-none" />
        ))}
      </div>
    );
  }

  if (kits.length === 0) {
    return (
      <div className="px-4 py-8 text-center md:px-6">
        <p className="text-sm text-bone-dim">No kits yet.</p>
        <p className="mx-auto mt-1 max-w-prose text-xs text-bone-faint">
          A kit is a basket of items with a name. Build one here and a player can ask for it by
          name, or load it into any give without picking through the catalogue again.
        </p>
        <Button variant="outline" size="sm" className="mt-3 gap-1" onClick={onBuild}>
          <Plus className="size-3" />
          Build one
        </Button>
      </div>
    );
  }

  return (
    <>
      <ul className="grid grid-cols-1 gap-px bg-border sm:grid-cols-2 lg:grid-cols-3">
        {kits.map((kit) => (
          <li key={kit.name} className="region p-3">
            <div className="flex items-baseline gap-2">
              <button
                type="button"
                className="readout min-w-0 flex-1 truncate text-left text-sm text-bone hover:text-ember"
                onClick={() => setEditing(kit)}
              >
                !kit {kit.name}
              </button>
              <span className="readout text-2xs text-bone-faint">
                {kit.items.reduce((sum, i) => sum + i.count, 0)}
              </span>
              <button
                type="button"
                className="text-bone-faint hover:text-crimson-lit"
                aria-label={`Delete the ${kit.name} kit`}
                onClick={() => setConfirming(kit)}
              >
                <Trash2 className="size-3" />
              </button>
            </div>
            <ul className="mt-2 flex flex-wrap gap-1">
              {kit.items.slice(0, 8).map((line) => (
                <li key={line.item} title={`${line.count} × ${line.item}`}>
                  <KitIcon name={line.item} count={line.count} />
                </li>
              ))}
              {kit.items.length > 8 && (
                <li className="readout self-center text-2xs text-bone-faint">
                  +{kit.items.length - 8}
                </li>
              )}
            </ul>
          </li>
        ))}
      </ul>

      {editing && <ItemPicker purpose="kit" editing={editing} onClose={() => setEditing(null)} />}

      <AlertDialog open={confirming !== null} onOpenChange={(open) => !open && setConfirming(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete the {confirming?.name} kit?</AlertDialogTitle>
            <AlertDialogDescription>
              Anybody who types <code className="readout">!kit {confirming?.name}</code> will be
              told there is no such kit.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Keep it</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => {
                if (!confirming) return;
                const name = confirming.name;
                remove.mutate(name, {
                  onSuccess: () => toast.success(`The ${name} kit is gone`),
                  onError: (err) => toast.error("Not deleted", { description: err.message }),
                });
                setConfirming(null);
              }}
            >
              Delete it
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}

/** One item in a kit, with how many of it. */
function KitIcon({ name, count }: { name: string; count: number }) {
  const serverId = useServerId();
  const [failed, setFailed] = useState(false);

  return (
    <span className="relative flex size-8 items-center justify-center border border-border">
      {failed ? (
        <span className="readout text-2xs text-bone-faint">?</span>
      ) : (
        <img
          src={`/api/servers/${serverId}/items/${encodeURIComponent(name)}/icon`}
          alt=""
          loading="lazy"
          decoding="async"
          className="size-6 object-contain"
          onError={() => setFailed(true)}
        />
      )}
      {count > 1 && (
        <span className="readout absolute -right-0.5 -bottom-1 bg-background px-0.5 text-2xs text-bone-dim">
          {count}
        </span>
      )}
    </span>
  );
}

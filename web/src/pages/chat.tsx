import { useState } from "react";
import { toast } from "sonner";
import { Plus, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
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
 * Five commands with three settings each is a small amount of information, and
 * the first pass still managed to fill a screen with it: every setting was a
 * row of chips, so switching a command on unfolded two more rows under it and
 * the page grew as it was configured. Now each command is one line — name,
 * controls, switch — and a second, quieter line saying what it does. A switch
 * changes what is on a line, not how many lines there are.
 *
 * The transcript sits beside that rather than above it. What is being
 * configured happens on somebody else's screen, so a picture of the result is
 * worth the space — but not the width of the page, and not the first thing
 * between an operator and the switches.
 */

/*
  Cooldowns as words.

  A number field asking for seconds is a small cruelty: 21600 is not a length
  of time anybody holds in their head. These are the lengths that get chosen,
  and anything else an operator has already set is shown alongside them.
*/
const COOLDOWNS = [
  { seconds: 0, label: "No limit" },
  { seconds: 10, label: "Every 10 seconds" },
  { seconds: 30, label: "Every 30 seconds" },
  { seconds: 300, label: "Every 5 minutes" },
  { seconds: 3600, label: "Once an hour" },
  { seconds: 21600, label: "Every 6 hours" },
  { seconds: 86400, label: "Once a day" },
];

function humanCooldown(seconds: number): string {
  const known = COOLDOWNS.find((c) => c.seconds === seconds);
  if (known) return known.label;
  if (seconds < 60) return `Every ${seconds} seconds`;
  if (seconds < 3600) return `Every ${Math.round(seconds / 60)} minutes`;
  return `Every ${Math.round(seconds / 360) / 10} hours`;
}

/** The example exchange each command is worth, for the transcript. */
const EXAMPLES: Record<string, { asked: string; answered: string }> = {
  help: { asked: "!help", answered: "You can use: !help, !day, !kit <name>" },
  day: { asked: "!day", answered: "Day 12, 14:30. Dark in 7h30m (19 minutes real)." },
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
          <Skeleton key={i} className="h-12 w-full rounded-none" />
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
  const anyAdminsOnly = on.some((c) => c.audience === "admins");

  return (
    <div className="min-h-0 flex-1 overflow-y-auto">
      <div className="flex flex-col lg:flex-row">
        <section className="min-w-0 flex-1">
          <div className="region-head">
            <span className="stencil">Commands</span>
            <span className="readout text-xs text-bone-faint">
              {on.length} of {data.commands.length} on
            </span>
            <span className="ml-auto hidden text-xs text-bone-faint sm:block">
              Off until you switch it on. None of them can kick, ban or wipe.
            </span>
          </div>

          <ul>
            {data.commands.map((command) => (
              <CommandRow key={command.name} command={command} kits={kits.data ?? []} />
            ))}
          </ul>

          {anyAdminsOnly && (
            <p className="border-b border-border px-4 py-2 text-2xs text-bone-faint md:px-6">
              Admins are whoever is on the game's own admin list, not who can sign in here.
            </p>
          )}
        </section>

        <Transcript prefix={data.prefix} enabled={on} />
      </div>

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
 * One command: one line to work with, one line to read.
 *
 * The controls appear only once it is on, in the space the row already has. A
 * switched-off command showing an audience and a cooldown invites the reading
 * that the settings are doing something, and they are not.
 */
function CommandRow({ command, kits }: { command: ChatCommand; kits: Kit[] }) {
  const save = useSaveChatCommand();

  const update = (change: Partial<ChatCommand>) =>
    save.mutate(
      {
        name: command.name,
        enabled: command.enabled,
        audience: command.audience,
        cooldownSeconds: command.cooldownSeconds,
        ...change,
      },
      { onError: (err) => toast.error("That did not save", { description: err.message }) },
    );

  // The one command that hands out loot, open to everybody, with no kits to
  // hand out: worth saying, because it will look broken from in game.
  const noKits = command.name === "kit" && command.enabled && kits.length === 0;

  return (
    <li
      className={cn(
        "border-b border-border px-4 py-2.5 md:px-6",
        command.enabled ? "bg-accent/20" : "text-bone-faint",
      )}
    >
      <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
        <code className={cn("readout text-sm", command.enabled ? "text-bone" : "text-bone-dim")}>
          {command.usage}
        </code>
        {command.acts && command.enabled && command.audience === "everyone" && (
          <span className="stencil text-ember">hands out loot</span>
        )}

        <div className="ml-auto flex items-center gap-2">
          {command.enabled && (
            <>
              <Picker
                label={`Who can use ${command.usage}`}
                value={command.audience}
                onChange={(audience) => update({ audience: audience as ChatAudience })}
                options={[
                  { value: "everyone", label: "Everyone" },
                  { value: "admins", label: "Admins only" },
                ]}
              />
              <Picker
                label={`How often ${command.usage} may be used`}
                value={String(command.cooldownSeconds)}
                onChange={(seconds) => update({ cooldownSeconds: Number(seconds) })}
                options={cooldownOptions(command.cooldownSeconds)}
              />
            </>
          )}
          <Switch
            checked={command.enabled}
            onCheckedChange={(enabled) => update({ enabled })}
            aria-label={`Answer ${command.usage}`}
          />
        </div>
      </div>

      <p className="mt-0.5 max-w-prose text-xs text-bone-faint">
        {command.summary}
        {noKits && (
          <span className="text-ember"> There are no kits yet, so it has none to give.</span>
        )}
      </p>
    </li>
  );
}

function Picker({
  label,
  value,
  onChange,
  options,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  options: { value: string; label: string }[];
}) {
  return (
    <Select value={value} onValueChange={onChange}>
      <SelectTrigger className="h-7 w-auto gap-1.5 px-2 text-xs" aria-label={label}>
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {options.map((option) => (
          <SelectItem key={option.value} value={option.value} className="text-xs">
            {option.label}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}

/**
 * The preset lengths, plus whatever this command is already set to.
 *
 * A value that is not a preset — set by an earlier version, or by hand — goes
 * in its place along the ramp rather than on the end, so the list stays an
 * order running short to long instead of an order with an odd one appended.
 */
function cooldownOptions(current: number) {
  const all = COOLDOWNS.some((c) => c.seconds === current)
    ? COOLDOWNS
    : [...COOLDOWNS, { seconds: current, label: humanCooldown(current) }].sort(
        (a, b) => a.seconds - b.seconds,
      );
  return all.map((c) => ({ value: String(c.seconds), label: c.label }));
}

/**
 * What a player sees, drawn as the game's own chat.
 *
 * Alongside the switches rather than above them, and only as wide as the lines
 * it holds: it is a reference while you work, not the headline.
 */
function Transcript({ prefix, enabled }: { prefix: string; enabled: ChatCommand[] }) {
  return (
    <aside className="shrink-0 border-b border-border lg:w-80 lg:border-b-0 lg:border-l xl:w-96">
      <div className="region-head">
        <span className="stencil">In game</span>
        <span className="readout text-xs text-bone-faint">{prefix}something</span>
      </div>

      <div className="p-4 md:px-6 lg:px-4">
        {enabled.length === 0 ? (
          <p className="text-xs text-bone-faint">
            Nothing is switched on, so the panel stays silent in chat. Turn one on to see what a
            player would get back.
          </p>
        ) : (
          <>
            <div className="space-y-2">
              {enabled.map((command) => {
                const example = EXAMPLES[command.name];
                if (!example) return null;
                return (
                  <div key={command.name} className="space-y-0.5 text-2xs">
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
            </div>
            <p className="mt-3 text-2xs text-bone-faint">
              The shape of the reply, not a live reading. Only the player who asked sees it.
            </p>
          </>
        )}
      </div>
    </aside>
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
          <Skeleton key={i} className="h-20 rounded-none" />
        ))}
      </div>
    );
  }

  if (kits.length === 0) {
    return (
      <div className="flex flex-wrap items-center gap-x-4 gap-y-2 px-4 py-4 md:px-6">
        <p className="max-w-prose text-xs text-bone-faint">
          A kit is a basket of items with a name. Build one and a player can ask for it by name, or
          load it into any give without picking through the catalogue again.
        </p>
        <Button variant="outline" size="sm" className="h-7 gap-1 text-xs" onClick={onBuild}>
          <Plus className="size-3" />
          Build one
        </Button>
      </div>
    );
  }

  return (
    <>
      {/*
        Tiles that stay their own size, rather than a grid of equal columns.
        With one kit saved, equal columns stretched it across a third of the
        page and painted the other two thirds in the colour of the gaps — a
        grey slab that looked like a rendering fault. auto-fill leaves the
        empty space empty.
      */}
      <ul className="grid grid-cols-[repeat(auto-fill,minmax(13rem,1fr))] gap-2 p-4 md:px-6">
        {kits.map((kit) => (
          <li key={kit.name} className="panel p-2.5">
            <div className="flex items-baseline gap-2">
              <button
                type="button"
                className="readout min-w-0 flex-1 truncate text-left text-xs text-bone hover:text-ember"
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
            <ul className="mt-2 flex flex-wrap gap-1.5">
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

/**
 * How many, in as few characters as fit under an icon.
 *
 * A kit of six thousand wood put "6000" under a 28px tile, which ran into the
 * number under the tile beside it and read as one long figure. Nobody needs
 * the last three digits of a stack count at a glance, and the exact figure is
 * still on the tile's own tooltip.
 */
function compactCount(n: number): string {
  if (n < 1000) return String(n);
  const thousands = n / 1000;
  return (thousands < 10 ? thousands.toFixed(1).replace(/\.0$/, "") : Math.round(thousands)) + "k";
}

/** One item in a kit, with how many of it. */
function KitIcon({ name, count }: { name: string; count: number }) {
  const serverId = useServerId();
  const [failed, setFailed] = useState(false);

  return (
    <span className="relative flex size-7 items-center justify-center border border-border">
      {failed ? (
        <span className="readout text-2xs text-bone-faint">?</span>
      ) : (
        <img
          src={`/api/servers/${serverId}/items/${encodeURIComponent(name)}/icon`}
          alt=""
          loading="lazy"
          decoding="async"
          className="size-5 object-contain"
          onError={() => setFailed(true)}
        />
      )}
      {count > 1 && (
        <span className="readout absolute -bottom-1.5 left-1/2 -translate-x-1/2 bg-background px-1 text-2xs text-bone-dim">
          {compactCount(count)}
        </span>
      )}
    </span>
  );
}

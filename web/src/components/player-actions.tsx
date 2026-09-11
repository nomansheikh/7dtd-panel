import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { api, type ActionResult, type BanUnit, type Player } from "@/lib/api";
import { useServerId } from "@/hooks/use-servers";

export type PendingAction = {
  kind: "teleport" | "give" | "xp" | "buff" | "kill" | "kick" | "ban" | "unban";
  player: Player;
};

const TITLES: Record<PendingAction["kind"], string> = {
  teleport: "Teleport",
  give: "Give an item",
  xp: "Give XP",
  buff: "Buff or debuff",
  kill: "Kill this player",
  kick: "Kick this player",
  ban: "Ban this player",
  unban: "Lift this ban",
};

/**
 * The dialog behind every per-player action.
 *
 * One component rather than eight, because they share the same shape: confirm
 * who it applies to, collect whatever the command needs, and hand back a
 * function that performs it. Nothing here builds a command string; the panel's
 * API does that with typed builders.
 */
export function PlayerActionDialog({
  pending,
  players,
  onClose,
  onRun,
}: {
  pending: PendingAction | null;
  players: Player[];
  onClose: () => void;
  onRun: (run: () => Promise<ActionResult>) => void;
}) {
  const serverId = useServerId();
  const [form, setForm] = useState<Record<string, string>>({});

  // Reset between openings, or the last action's values leak into the next.
  useEffect(() => {
    if (!pending) return;
    setForm(
      pending.kind === "give"
        ? { count: "1", quality: "0" }
        : pending.kind === "xp"
          ? { amount: "1000" }
          : pending.kind === "ban"
            ? { duration: "1", unit: "days" }
            : pending.kind === "teleport"
              ? { mode: "coords", x: "0", y: "-1", z: "0" }
              : {},
    );
  }, [pending]);

  if (!pending) return null;
  const { kind, player } = pending;
  const set = (key: string) => (value: string) => setForm((f) => ({ ...f, [key]: value }));

  const destructive = kind === "kill" || kind === "kick" || kind === "ban";

  function build(): (() => Promise<ActionResult>) | null {
    switch (kind) {
      case "teleport":
        if (form.mode === "player") {
          const to = Number(form.toEntityId);
          if (!Number.isFinite(to)) return null;
          return () => api.teleport(serverId, player.entityId, { toEntityId: to });
        }
        return () =>
          api.teleport(serverId, player.entityId, {
            x: Number(form.x),
            y: Number(form.y),
            z: Number(form.z),
          });
      case "give":
        if (!form.item) return null;
        return () =>
          api.giveItem(
            serverId,
            player.entityId,
            form.item,
            Number(form.count || 1),
            Number(form.quality || 0),
          );
      case "xp":
        return () => api.giveXP(serverId, player.entityId, Number(form.amount || 0));
      case "buff":
        if (!form.buff) return null;
        return () => api.buffPlayer(serverId, player.entityId, form.buff, form.remove === "yes");
      case "kill":
        return () => api.killPlayer(serverId, player.entityId);
      case "kick":
        return () => api.kickPlayer(serverId, player.entityId, form.reason ?? "");
      case "ban":
        return () =>
          api.banPlayer(
            serverId,
            player.platformId,
            Number(form.duration || 1),
            (form.unit as BanUnit) ?? "days",
            form.reason ?? "",
          );
      case "unban":
        return () => api.unbanPlayer(serverId, player.platformId);
    }
  }

  const run = build();

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{TITLES[kind]}</DialogTitle>
          <DialogDescription>
            {kind === "ban" || kind === "unban"
              ? `${player.name} — ${player.platformId}`
              : `${player.name} — entity ${player.entityId}`}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          {kind === "teleport" && (
            <TeleportFields
              form={form}
              set={set}
              players={players.filter((p) => p.online && p.entityId !== player.entityId)}
            />
          )}

          {kind === "give" && <GiveFields form={form} set={set} />}

          {kind === "xp" && (
            <Field label="Amount" htmlFor="xp-amount">
              <Input
                id="xp-amount"
                inputMode="numeric"
                value={form.amount ?? ""}
                onChange={(e) => set("amount")(e.target.value)}
              />
              <p className="mt-1 text-xs text-muted-foreground">
                The game has no command to set a level, only to grant XP, so this cannot lower one.
              </p>
            </Field>
          )}

          {kind === "buff" && (
            <>
              <Field label="Buff name" htmlFor="buff-name">
                <Input
                  id="buff-name"
                  value={form.buff ?? ""}
                  onChange={(e) => set("buff")(e.target.value)}
                  placeholder="buffInjuryDeepLaceration"
                  className="font-mono"
                />
              </Field>
              <Field label="Action" htmlFor="buff-mode">
                <Select value={form.remove ?? "no"} onValueChange={set("remove")}>
                  <SelectTrigger id="buff-mode">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="no">Apply</SelectItem>
                    <SelectItem value="yes">Remove</SelectItem>
                  </SelectContent>
                </Select>
              </Field>
            </>
          )}

          {kind === "kick" && (
            <Field label="Reason (optional)" htmlFor="kick-reason">
              <Input
                id="kick-reason"
                value={form.reason ?? ""}
                onChange={(e) => set("reason")(e.target.value)}
              />
            </Field>
          )}

          {kind === "ban" && <BanFields form={form} set={set} />}

          {kind === "kill" && (
            <p className="text-sm text-muted-foreground">
              They will drop their backpack where they stand, subject to the server's death
              settings.
            </p>
          )}

          {kind === "unban" && (
            <p className="text-sm text-muted-foreground">
              They will be able to reconnect immediately.
            </p>
          )}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={onClose}>
            Cancel
          </Button>
          <Button
            variant={destructive ? "destructive" : "default"}
            disabled={!run}
            onClick={() => run && onRun(run)}
          >
            {TITLES[kind]}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function Field({
  label,
  htmlFor,
  children,
}: {
  label: string;
  htmlFor: string;
  children: React.ReactNode;
}) {
  return (
    <div className="space-y-1">
      <Label htmlFor={htmlFor}>{label}</Label>
      {children}
    </div>
  );
}

function TeleportFields({
  form,
  set,
  players,
}: {
  form: Record<string, string>;
  set: (key: string) => (value: string) => void;
  players: Player[];
}) {
  return (
    <>
      <Field label="Destination" htmlFor="tp-mode">
        <Select value={form.mode ?? "coords"} onValueChange={set("mode")}>
          <SelectTrigger id="tp-mode">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="coords">Coordinates</SelectItem>
            <SelectItem value="player" disabled={players.length === 0}>
              Another player
            </SelectItem>
          </SelectContent>
        </Select>
      </Field>

      {form.mode === "player" ? (
        <Field label="Player" htmlFor="tp-target">
          <Select value={form.toEntityId ?? ""} onValueChange={set("toEntityId")}>
            <SelectTrigger id="tp-target">
              <SelectValue placeholder="Choose someone online" />
            </SelectTrigger>
            <SelectContent>
              {players.map((p) => (
                <SelectItem key={p.entityId} value={String(p.entityId)}>
                  {p.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </Field>
      ) : (
        <div className="flex gap-2">
          {(["x", "y", "z"] as const).map((axis) => (
            <div key={axis} className="flex-1 space-y-1">
              <Label htmlFor={`tp-${axis}`}>{axis.toUpperCase()}</Label>
              <Input
                id={`tp-${axis}`}
                inputMode="numeric"
                value={form[axis] ?? ""}
                onChange={(e) => set(axis)(e.target.value)}
              />
            </div>
          ))}
        </div>
      )}
      {form.mode !== "player" && (
        <p className="text-xs text-muted-foreground">
          Y of −1 drops them onto the ground rather than a precise height.
        </p>
      )}
    </>
  );
}

function GiveFields({
  form,
  set,
}: {
  form: Record<string, string>;
  set: (key: string) => (value: string) => void;
}) {
  const serverId = useServerId();
  const [query, setQuery] = useState("");

  // Searched on the panel: the full catalogue is about 2.9 MB and thousands of
  // entries, so it is never shipped to the browser.
  const { data } = useQuery({
    queryKey: ["items", serverId, query],
    queryFn: () => api.searchItems(serverId, query),
    enabled: query.trim().length > 1,
  });

  return (
    <>
      <Field label="Search items" htmlFor="give-search">
        <Input
          id="give-search"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="axe, wood, ammo…"
        />
      </Field>

      <Field label="Item" htmlFor="give-item">
        <Select value={form.item ?? ""} onValueChange={set("item")}>
          <SelectTrigger id="give-item">
            <SelectValue
              placeholder={
                query.trim().length > 1
                  ? `${data?.total ?? 0} matches`
                  : "Type at least two letters"
              }
            />
          </SelectTrigger>
          <SelectContent>
            {(data?.items ?? []).map((item) => (
              <SelectItem key={item.name} value={item.name}>
                {item.localizedName || item.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </Field>

      <div className="flex gap-2">
        <div className="flex-1 space-y-1">
          <Label htmlFor="give-count">Count</Label>
          <Input
            id="give-count"
            inputMode="numeric"
            value={form.count ?? ""}
            onChange={(e) => set("count")(e.target.value)}
          />
        </div>
        <div className="flex-1 space-y-1">
          <Label htmlFor="give-quality">Quality (0 for none)</Label>
          <Input
            id="give-quality"
            inputMode="numeric"
            value={form.quality ?? ""}
            onChange={(e) => set("quality")(e.target.value)}
          />
        </div>
      </div>
      <p className="text-xs text-muted-foreground">
        The item is dropped in front of the player rather than placed in their inventory.
      </p>
    </>
  );
}

function BanFields({
  form,
  set,
}: {
  form: Record<string, string>;
  set: (key: string) => (value: string) => void;
}) {
  return (
    <>
      <div className="flex gap-2">
        <div className="w-24 space-y-1">
          <Label htmlFor="ban-duration">Duration</Label>
          <Input
            id="ban-duration"
            inputMode="numeric"
            value={form.duration ?? ""}
            onChange={(e) => set("duration")(e.target.value)}
          />
        </div>
        <div className="flex-1 space-y-1">
          <Label htmlFor="ban-unit">Unit</Label>
          <Select value={form.unit ?? "days"} onValueChange={set("unit")}>
            <SelectTrigger id="ban-unit">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {["minutes", "hours", "days", "weeks", "months", "years"].map((u) => (
                <SelectItem key={u} value={u}>
                  {u}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>
      <Field label="Reason (optional)" htmlFor="ban-reason">
        <Input
          id="ban-reason"
          value={form.reason ?? ""}
          onChange={(e) => set("reason")(e.target.value)}
        />
      </Field>
      <p className="text-xs text-muted-foreground">
        Bans are by platform id, so they hold after the player disconnects. Quotes and line breaks
        are rejected in the reason.
      </p>
    </>
  );
}

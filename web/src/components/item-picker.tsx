import { useEffect, useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { toast } from "sonner";
import { ChevronDown, Minus, Plus, Search, Trash2 } from "lucide-react";
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
import { Skeleton } from "@/components/ui/skeleton";
import { useServerId } from "@/hooks/use-servers";
import { useDeleteKit, useKits, useSaveKit } from "@/hooks/use-chat";
import { api, type GameItem, type Kit } from "@/lib/api";
import { cn } from "@/lib/utils";

/**
 * Picking things to hand somebody.
 *
 * What this replaces was a search box that refused to show anything until you
 * had typed two letters, feeding a dropdown. That is only usable by somebody
 * who already knows what the game calls the thing they want — and the game
 * calls a hunting knife `meleeWpnBladeT1HuntingKnife`. You cannot search a
 * catalogue you have never seen, so this one is built to be looked through
 * rather than queried: the whole list arrives at once, in categories, with the
 * game's own artwork.
 *
 * It also gives more than one thing at a time. `give` takes a single item, so
 * the panel composes: a basket of items becomes a run of commands, and a
 * basket worth keeping becomes a kit. That is the part the game's API does not
 * do and the panel can.
 *
 * The same sheet builds kits, because it is the same act. A kit was briefly
 * kept in local storage, on the reasoning that it was one operator's habit
 * rather than server state — which stopped being true the moment the chat bot
 * could hand one over at three in the morning with no browser open anywhere.
 */

/** One line of a delivery. */
export interface Pick {
  item: GameItem;
  count: number;
  quality: number;
}

/**
 * How good the thing is when it arrives.
 *
 * Nought means the panel leaves the argument off entirely and the game decides;
 * one to six is the game's own scale, worst to best.
 *
 * The words are the panel's, and the number is kept beside each one rather than
 * replaced by it. That pairing is the whole point: nothing the server exposes
 * carries tier names — /api/item returns a name, a localised name and whether
 * it is a block — so a word on its own would be invented vocabulary dressed up
 * as the game's, and an operator comparing it to a wiki would find nothing.
 * Six bare digits told them nothing either. Both together are honest and
 * readable, and the number is what ends up in the command.
 *
 * In a dropdown rather than a row of buttons: seven of these under every line
 * of the basket was most of the sheet, and six of the seven were always the
 * wrong answer.
 */
const QUALITIES = [
  { value: 0, label: "Any quality" },
  { value: 1, label: "Crude · 1" },
  { value: 2, label: "Basic · 2" },
  { value: 3, label: "Decent · 3" },
  { value: 4, label: "Good · 4" },
  { value: 5, label: "Fine · 5" },
  { value: 6, label: "Flawless · 6" },
];

/*
  Categories, off the naming convention.

  The order matters: `meleeTool` has to be tested before `melee`, or every axe
  files itself under weapons.
*/
const CATEGORIES: { id: string; label: string; match: (name: string) => boolean }[] = [
  { id: "weapons", label: "Weapons", match: (n) => /^(meleeWpn|gun|thrown|bow)/.test(n) },
  { id: "ammo", label: "Ammo", match: (n) => n.startsWith("ammo") },
  { id: "tools", label: "Tools", match: (n) => /^(meleeTool|tool|bucket)/.test(n) },
  { id: "armor", label: "Armour", match: (n) => /^(armor|clothing)/.test(n) },
  { id: "food", label: "Food", match: (n) => /^(food|drink)/.test(n) },
  { id: "medical", label: "Medical", match: (n) => /^(medical|drug)/.test(n) },
  { id: "resources", label: "Resources", match: (n) => n.startsWith("resource") },
  { id: "mods", label: "Mods", match: (n) => n.startsWith("mod") },
  { id: "books", label: "Books", match: (n) => /^(book|schematic|note|perk)/.test(n) },
  { id: "vehicles", label: "Vehicles", match: (n) => n.startsWith("vehicle") },
];

function categoryOf(name: string): string {
  return CATEGORIES.find((c) => c.match(name))?.id ?? "other";
}

/**
 * Whether the game has a real name for this, as opposed to an internal id.
 *
 * Roughly a quarter of the catalogue is scaffolding — loot tiers, quest
 * plumbing, dev tickets — and the tell is that the game never wrote a display
 * name for it, so the localised name comes back identical to the internal one.
 * Hiding those is the difference between browsing fifteen hundred entries and
 * browsing the thousand a person might actually want to hand over.
 */
function hasRealName(item: GameItem): boolean {
  return Boolean(item.localizedName) && item.localizedName !== item.name;
}

/** The game's own art for an item, proxied by the panel. */
function ItemIcon({ name, className }: { name: string; className?: string }) {
  const serverId = useServerId();
  const [failed, setFailed] = useState(false);

  if (failed) {
    return (
      <div
        className={cn("flex items-center justify-center bg-bone/5 text-bone-faint", className)}
        aria-hidden
      >
        <span className="readout text-2xs">?</span>
      </div>
    );
  }
  return (
    <img
      src={`/api/servers/${serverId}/items/${encodeURIComponent(name)}/icon`}
      alt=""
      loading="lazy"
      decoding="async"
      className={cn("object-contain", className)}
      onError={() => setFailed(true)}
    />
  );
}

interface Props {
  /**
   * What the basket is for: handing over now, or saving under a name.
   *
   * The browsing, the basket and the quality controls are identical either
   * way — only the last step differs — so this is a flag rather than a second
   * component that would drift from this one within a release.
   */
  purpose?: "give" | "kit";
  /** Who it is going to, for the wording on the button. */
  recipient?: string;
  /** How many are online, for the fan-out. */
  alsoOnline?: number;
  onGive?: (picks: Pick[], everyone: boolean) => void;
  /** The kit being changed, when there is one. Its name is then fixed. */
  editing?: Kit;
  onClose: () => void;
}

export function ItemPicker({
  purpose = "give",
  recipient = "",
  alsoOnline = 0,
  onGive,
  editing,
  onClose,
}: Props) {
  const serverId = useServerId();
  const [query, setQuery] = useState("");
  const [category, setCategory] = useState("all");
  const [showInternal, setShowInternal] = useState(false);
  const [basket, setBasket] = useState<Pick[]>([]);
  const [basketOpen, setBasketOpen] = useState(true);
  const [kitName, setKitName] = useState(editing?.name ?? "");

  const kits = useKits();
  const saveKit = useSaveKit();
  const deleteKit = useDeleteKit();

  // The whole catalogue, once. It is 128 kB and does not change while the
  // server is up, so filtering it here is instant and browsing it is possible
  // at all.
  const { data, isLoading } = useQuery({
    queryKey: ["items", serverId, "all"],
    queryFn: () => api.allItems(serverId),
    enabled: serverId !== "",
    staleTime: 30 * 60 * 1000,
  });

  const items = useMemo(() => {
    const all = data?.items ?? [];
    return showInternal ? all : all.filter(hasRealName);
  }, [data, showInternal]);

  /**
   * Turns a saved kit back into a basket.
   *
   * A kit stores only the item's internal name, because that is what the give
   * command takes and the only part that is stable. Everything else — the
   * display name, the artwork — is looked up in the catalogue that is already
   * loaded. An item that is no longer in the catalogue still comes back, under
   * its internal name, rather than quietly vanishing from a kit that somebody
   * is still handing out.
   */
  const asBasket = useMemo(
    () =>
      (kit: Kit): Pick[] => {
        const catalogue = new Map((data?.items ?? []).map((item) => [item.name, item]));
        return kit.items.map((line) => ({
          item: catalogue.get(line.item) ?? {
            name: line.item,
            localizedName: line.item,
            isBlock: false,
          },
          count: line.count,
          quality: line.quality,
        }));
      },
    [data],
  );

  // A kit opened for editing fills the basket once the catalogue has arrived,
  // so its rows carry names and artwork rather than raw ids.
  useEffect(() => {
    if (editing && data) setBasket(asBasket(editing));
  }, [editing, data, asBasket]);

  const counts = useMemo(() => {
    const out: Record<string, number> = { all: items.length };
    for (const item of items) {
      const id = categoryOf(item.name);
      out[id] = (out[id] ?? 0) + 1;
    }
    return out;
  }, [items]);

  const needle = query.trim().toLowerCase();
  const shown = useMemo(
    () =>
      items.filter(
        (item) =>
          // A search runs across every category, because somebody typing
          // "knife" does not know which bucket the game filed it under.
          (needle !== "" || category === "all" || categoryOf(item.name) === category) &&
          (needle === "" ||
            item.localizedName.toLowerCase().includes(needle) ||
            item.name.toLowerCase().includes(needle)),
      ),
    [items, needle, category],
  );

  // Capped for the browser's sake: a thousand images at once is a stutter, and
  // nobody scrolls past two hundred anyway — they narrow instead.
  const visible = shown.slice(0, 200);

  const add = (item: GameItem) =>
    setBasket((current) => {
      const at = current.findIndex((p) => p.item.name === item.name);
      if (at === -1) return [...current, { item, count: 1, quality: 0 }];
      const next = [...current];
      next[at] = { ...next[at], count: next[at].count + 1 };
      return next;
    });

  const update = (name: string, change: Partial<Pick>) =>
    setBasket((current) => current.map((p) => (p.item.name === name ? { ...p, ...change } : p)));

  const total = basket.reduce((sum, p) => sum + p.count, 0);

  return (
    <Sheet open onOpenChange={(open) => !open && onClose()}>
      {/*
        Wide for a sheet. The whole point is seeing a grid of artwork at once,
        and at the default width that is two icons a row — which is a list
        again, with pictures.
      */}
      <SheetContent side="right" className="w-full gap-0 p-0 sm:max-w-3xl">
        <SheetHeader className="region-head shrink-0 space-y-0 p-3 md:px-4">
          <SheetTitle className="stencil">
            {purpose === "kit"
              ? editing
                ? `Edit the ${editing.name} kit`
                : "Build a kit"
              : `Give to ${recipient}`}
          </SheetTitle>
          <SheetDescription className="sr-only">
            Browse the catalogue and build a basket{" "}
            {purpose === "kit" ? "to save under a name." : "to hand over."}
          </SheetDescription>
        </SheetHeader>

        <div className="flex shrink-0 flex-wrap items-center gap-2 border-b border-border p-3 md:px-4">
          <div className="relative min-w-48 flex-1">
            <Search className="absolute top-1/2 left-2 size-3.5 -translate-y-1/2 text-bone-faint" />
            <Input
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="Search by the name it has in game"
              className="h-8 pl-7"
              autoFocus
            />
          </div>
          <span className="readout text-xs text-bone-faint">
            {shown.length === items.length ? items.length : `${shown.length} of ${items.length}`}
          </span>
          <Button
            variant="ghost"
            size="sm"
            className="h-8 px-2 text-xs text-bone-dim"
            onClick={() => setShowInternal((v) => !v)}
          >
            {showInternal ? "Hide internal" : "Show internal"}
          </Button>
        </div>

        {!needle && (
          <div className="flex shrink-0 flex-wrap gap-1 border-b border-border p-3 md:px-4">
            {[{ id: "all", label: "Everything" }, ...CATEGORIES, { id: "other", label: "Other" }]
              .filter((c) => c.id === "all" || (counts[c.id] ?? 0) > 0)
              .map((c) => (
                <button
                  key={c.id}
                  type="button"
                  onClick={() => setCategory(c.id)}
                  className={cn(
                    "flex items-baseline gap-1.5 border px-2.5 py-1 text-xs transition-colors",
                    c.id === category
                      ? "border-border bg-accent text-bone"
                      : "border-transparent text-bone-dim hover:bg-accent/50",
                  )}
                >
                  {c.label}
                  <span className="readout text-2xs text-bone-faint">{counts[c.id] ?? 0}</span>
                </button>
              ))}
          </div>
        )}

        {isLoading ? (
          <div className="grid grid-cols-3 gap-2 p-3 sm:grid-cols-5">
            {Array.from({ length: 15 }, (_, i) => (
              <Skeleton key={i} className="h-24 rounded-none" />
            ))}
          </div>
        ) : (
          <ul className="grid min-h-0 flex-1 grid-cols-3 content-start gap-2 overflow-y-auto p-3 sm:grid-cols-5 md:px-4">
            {visible.map((item) => (
              <li key={item.name}>
                <button
                  type="button"
                  onClick={() => add(item)}
                  title={item.name}
                  className="flex h-full w-full flex-col items-center gap-1.5 border border-border p-2 text-center transition-colors hover:bg-accent"
                >
                  <ItemIcon name={item.name} className="size-10" />
                  <span className="line-clamp-2 text-xs text-bone-dim">{item.localizedName}</span>
                </button>
              </li>
            ))}
            {shown.length > visible.length && (
              <li className="col-span-full p-2 text-center text-xs text-bone-faint">
                {shown.length - visible.length} more — narrow it with the search or a category.
              </li>
            )}
            {shown.length === 0 && (
              <li className="col-span-full p-6 text-center text-xs text-bone-faint">
                Nothing matches.
              </li>
            )}
          </ul>
        )}

        {/*
          The basket, along the bottom rather than in a second column: a sheet
          is not wide enough to give one away, and what is in it matters far
          less often than what is going into it.
        */}
        <div className="shrink-0 border-t border-border">
          <button
            type="button"
            className="region-head w-full text-left"
            onClick={() => setBasketOpen((v) => !v)}
          >
            <span className="stencil">Basket</span>
            <span className="readout text-xs text-bone-faint">
              {total === 0 ? "empty" : `${total} in ${basket.length}`}
            </span>
            {basket.length > 0 && (
              <ChevronDown
                className={cn(
                  "ml-auto size-3.5 text-bone-faint transition-transform",
                  basketOpen && "rotate-180",
                )}
              />
            )}
          </button>

          {basket.length > 0 && basketOpen && (
            <ul className="max-h-56 overflow-y-auto border-t border-border">
              {basket.map((pick) => (
                <li key={pick.item.name} className="border-b border-border/60 px-3 py-2 md:px-4">
                  <div className="flex flex-wrap items-center gap-2">
                    <ItemIcon name={pick.item.name} className="size-6 shrink-0" />
                    <span className="min-w-0 flex-1 truncate text-xs text-bone">
                      {pick.item.localizedName}
                    </span>

                    <div className="flex items-center gap-1">
                      <Button
                        variant="ghost"
                        size="icon"
                        className="size-6"
                        onClick={() =>
                          update(pick.item.name, { count: Math.max(1, pick.count - 1) })
                        }
                        aria-label="One fewer"
                      >
                        <Minus className="size-3" />
                      </Button>
                      <Input
                        inputMode="numeric"
                        value={String(pick.count)}
                        onChange={(e) =>
                          update(pick.item.name, {
                            count: Math.max(1, Number(e.target.value) || 1),
                          })
                        }
                        className="h-6 w-14 text-center text-xs"
                        aria-label={`How many ${pick.item.localizedName}`}
                      />
                      <Button
                        variant="ghost"
                        size="icon"
                        className="size-6"
                        onClick={() => update(pick.item.name, { count: pick.count + 1 })}
                        aria-label="One more"
                      >
                        <Plus className="size-3" />
                      </Button>
                    </div>

                    <Select
                      value={String(pick.quality)}
                      onValueChange={(quality) =>
                        update(pick.item.name, { quality: Number(quality) })
                      }
                    >
                      <SelectTrigger
                        className="h-6 w-auto gap-1.5 px-2 text-xs"
                        aria-label={`Quality of ${pick.item.localizedName}`}
                      >
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {QUALITIES.map((q) => (
                          <SelectItem key={q.value} value={String(q.value)} className="text-xs">
                            {q.label}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>

                    <Button
                      variant="ghost"
                      size="icon"
                      className="size-6 text-bone-faint"
                      onClick={() =>
                        setBasket((c) => c.filter((p) => p.item.name !== pick.item.name))
                      }
                      aria-label={`Take ${pick.item.localizedName} out`}
                    >
                      <Trash2 className="size-3" />
                    </Button>
                  </div>
                </li>
              ))}
            </ul>
          )}
        </div>

        {/*
          Kits: a basket worth keeping, saved on the panel rather than in this
          browser, because the chat bot hands them out too.
        */}
        {((kits.data?.length ?? 0) > 0 || basket.length > 0) && (
          <div className="shrink-0 space-y-2 border-t border-border p-3 md:px-4">
            {(kits.data?.length ?? 0) > 0 && !editing && (
              <div className="flex flex-wrap items-center gap-1">
                <span className="stencil mr-1">Kits</span>
                {kits.data?.map((kit) => (
                  <span key={kit.name} className="flex items-center border border-border">
                    <button
                      type="button"
                      className="px-2 py-1 text-xs text-bone-dim hover:text-bone"
                      onClick={() => setBasket(asBasket(kit))}
                    >
                      {kit.name}
                      <span className="readout ml-1.5 text-2xs text-bone-faint">
                        {kit.items.reduce((sum, line) => sum + line.count, 0)}
                      </span>
                    </button>
                    <button
                      type="button"
                      className="px-1.5 py-1 text-bone-faint hover:text-crimson-lit"
                      aria-label={`Delete the ${kit.name} kit`}
                      onClick={() =>
                        deleteKit.mutate(kit.name, {
                          onError: (err) =>
                            toast.error("Not deleted", { description: err.message }),
                        })
                      }
                    >
                      <Trash2 className="size-3" />
                    </button>
                  </span>
                ))}
              </div>
            )}

            {basket.length > 0 && (
              <div className="space-y-1">
                <div className="flex gap-2">
                  <Input
                    value={kitName}
                    onChange={(e) => setKitName(e.target.value.toLowerCase())}
                    placeholder="Save this basket as a kit"
                    className="h-7 flex-1 text-xs"
                    disabled={Boolean(editing)}
                  />
                  <Button
                    variant={purpose === "kit" ? "default" : "outline"}
                    size="sm"
                    className="h-7"
                    disabled={kitName.trim() === "" || saveKit.isPending}
                    onClick={() => {
                      const name = kitName.trim();
                      saveKit.mutate(
                        {
                          name,
                          items: basket.map((pick) => ({
                            item: pick.item.name,
                            count: pick.count,
                            quality: pick.quality,
                          })),
                        },
                        {
                          onSuccess: () => {
                            toast.success(`Saved as !kit ${name}`, {
                              description:
                                "Players can ask for it by name once !kit is switched on.",
                            });
                            if (purpose === "kit") onClose();
                            else setKitName("");
                          },
                          onError: (err) => toast.error("Not saved", { description: err.message }),
                        },
                      );
                    }}
                  >
                    {editing ? "Save changes" : "Save kit"}
                  </Button>
                </div>
                <p className="text-2xs text-bone-faint">
                  Lowercase letters, digits, dash and underscore — it has to be typeable in chat.
                </p>
              </div>
            )}
          </div>
        )}

        {purpose === "give" && onGive && (
          <div className="flex shrink-0 gap-2 border-t border-border p-3 md:px-4">
            <Button
              className="flex-1"
              disabled={basket.length === 0}
              onClick={() => onGive(basket, false)}
            >
              Give to {recipient}
            </Button>
            {alsoOnline > 1 && (
              <Button
                variant="outline"
                disabled={basket.length === 0}
                onClick={() => onGive(basket, true)}
              >
                Everyone online ({alsoOnline})
              </Button>
            )}
          </div>
        )}
      </SheetContent>
    </Sheet>
  );
}

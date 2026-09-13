import { useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { useMutation, useQuery } from "@tanstack/react-query";
import {
  columnFilteringFeature,
  createFilteredRowModel,
  createSortedRowModel,
  filterFns,
  flexRender,
  globalFilteringFeature,
  rowSortingFeature,
  sortFns,
  tableFeatures,
  useTable,
  type ColumnDef,
  type SortingState,
} from "@tanstack/react-table";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { PlayerActionDialog, type PendingAction } from "@/components/player-actions";
import { usePlayerAction, usePlayers } from "@/hooks/use-players";
import { useServerId } from "@/hooks/use-servers";
import { api, ApiError, type Player } from "@/lib/api";
import { formatAge, formatUptime } from "@/lib/format";

function lastSeen(player: Player): string {
  if (player.online) return "now";
  if (!player.lastOnline) return "never";
  return formatAge((Date.now() - Date.parse(player.lastOnline)) / 1000);
}

// react-table v9 is feature-based: only the features declared here are wired
// into the table, which keeps the bundle to what is actually used.
//
// Row models and the named sort/filter functions are registered here too
// rather than passed to useTable: in v9 createSortedRowModel and
// createFilteredRowModel take no arguments, and globalFilteringFeature will
// not run without columnFilteringFeature alongside it.
const features = tableFeatures({
  rowSortingFeature,
  columnFilteringFeature,
  globalFilteringFeature,
  sortedRowModel: createSortedRowModel(),
  filteredRowModel: createFilteredRowModel(),
  sortFns,
  filterFns,
});

export function PlayersPage() {
  const { data, isLoading } = usePlayers();
  const action = usePlayerAction();
  const [filter, setFilter] = useState("");
  const [sorting, setSorting] = useState<SortingState>([]);
  const [pending, setPending] = useState<PendingAction | null>(null);
  const [showClaims, setShowClaims] = useState(false);
  const [kickingAll, setKickingAll] = useState(false);

  const players = useMemo(() => data?.players ?? [], [data]);

  const columns = useMemo<ColumnDef<typeof features, Player, unknown>[]>(
    () => [
      {
        accessorKey: "name",
        header: "Player",
        cell: ({ row }) => (
          <Link
            to={`/players/${encodeURIComponent(row.original.platformId)}`}
            className="flex items-center gap-2 hover:text-bone"
          >
            <span
              aria-hidden
              className={
                row.original.online
                  ? "size-2 shrink-0 rounded-full bg-status-online"
                  : "size-2 shrink-0 rounded-full bg-status-unknown"
              }
            />
            <span className="font-medium underline-offset-4 hover:underline">
              {row.original.name}
            </span>
            {row.original.banned && <Badge variant="destructive">banned</Badge>}
          </Link>
        ),
      },
      {
        id: "lastSeen",
        header: "Last seen",
        accessorFn: (p) => (p.online ? Infinity : Date.parse(p.lastOnline ?? "0")),
        cell: ({ row }) => (
          <span className="readout text-muted-foreground">{lastSeen(row.original)}</span>
        ),
      },
      {
        accessorKey: "playTimeSeconds",
        header: "This life",
        cell: ({ getValue }) => <span className="readout">{formatUptime(getValue<number>())}</span>,
      },
      {
        accessorKey: "level",
        header: "Level",
        // Level, health and the counters come from the online-players endpoint,
        // which knows nothing about anyone who has logged off.
        cell: ({ row }) =>
          row.original.online ? (
            <span className="readout">{row.original.level}</span>
          ) : (
            <span className="text-muted-foreground">—</span>
          ),
      },
      {
        accessorKey: "health",
        header: "Health",
        cell: ({ row }) =>
          row.original.online ? (
            <span className="readout">{row.original.health}</span>
          ) : (
            <span className="text-muted-foreground">—</span>
          ),
      },
      {
        accessorKey: "deaths",
        header: "Deaths",
        cell: ({ row }) =>
          row.original.online ? (
            <span className="readout">{row.original.deaths}</span>
          ) : (
            <span className="text-muted-foreground">—</span>
          ),
      },
      {
        accessorKey: "zombieKills",
        header: "Zombies",
        cell: ({ row }) =>
          row.original.online ? (
            <span className="readout">{row.original.zombieKills}</span>
          ) : (
            <span className="text-muted-foreground">—</span>
          ),
      },
      {
        accessorKey: "ping",
        header: "Ping",
        cell: ({ row }) =>
          row.original.online ? (
            <span className="readout">{row.original.ping}</span>
          ) : (
            <span className="text-muted-foreground">—</span>
          ),
      },
      {
        id: "position",
        header: "Position",
        cell: ({ row }) => {
          const p = row.original.position;
          if (!p) return <span className="text-muted-foreground">—</span>;
          return (
            <span className="readout text-muted-foreground">
              {Math.round(p.x)}, {Math.round(p.y)}, {Math.round(p.z)}
            </span>
          );
        },
      },
    ],
    [],
  );

  const table = useTable({
    features,
    data: players,
    columns,
    state: { sorting, globalFilter: filter },
    onSortingChange: setSorting,
    onGlobalFilterChange: setFilter,
  });

  const online = players.filter((p) => p.online).length;

  return (
    <div className="flex h-full min-h-0 flex-col">
      {/* Wraps rather than squashing: with no room the count was breaking into
          a column of fragments down the left edge. */}
      <div className="region-head shrink-0 flex-wrap gap-3">
        <span className="stencil shrink-0">
          {online} online · {players.length} known
        </span>
        <Input
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
          placeholder="Filter by name"
          className="order-last h-7 w-full text-xs sm:order-none sm:ml-auto sm:w-auto sm:max-w-56"
        />
        {/* Two things that are about everybody rather than about one row, so
            they sit in the header instead of in every player's menu. */}
        <Button
          variant="ghost"
          size="sm"
          className="h-7 shrink-0 px-2 text-xs text-bone-dim"
          onClick={() => setShowClaims(true)}
        >
          Land claims
        </Button>
        <Button
          variant="ghost"
          size="sm"
          disabled={online === 0}
          className="h-7 shrink-0 px-2 text-xs text-bone-dim"
          onClick={() => setKickingAll(true)}
        >
          Kick everyone
        </Button>
      </div>

      {isLoading && !data ? (
        <Skeleton className="m-4 h-64 rounded-none" />
      ) : players.length === 0 ? (
        <p className="flex flex-1 items-center justify-center p-8 text-center text-sm text-bone-faint">
          Nobody has joined this server yet. Players appear here as soon as they connect, and stay
          listed after they leave.
        </p>
      ) : (
        <div className="region min-h-0 flex-1 overflow-auto">
          <Table>
            <TableHeader>
              {table.getHeaderGroups().map((group) => (
                <TableRow key={group.id}>
                  {group.headers.map((header) => (
                    <TableHead key={header.id}>
                      {header.isPlaceholder ? null : (
                        <button
                          type="button"
                          className="flex items-center gap-1"
                          onClick={header.column.getToggleSortingHandler()}
                        >
                          {flexRender(header.column.columnDef.header, header.getContext())}
                          {{ asc: "↑", desc: "↓" }[header.column.getIsSorted() as string] ?? null}
                        </button>
                      )}
                    </TableHead>
                  ))}
                </TableRow>
              ))}
            </TableHeader>
            <TableBody>
              {table.getRowModel().rows.map((row) => (
                <TableRow key={row.id}>
                  {row.getAllCells().map((cell) => (
                    <TableCell key={cell.id}>
                      {flexRender(cell.column.columnDef.cell, cell.getContext())}
                    </TableCell>
                  ))}
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}

      <p className="text-xs text-muted-foreground">
        Level, health, deaths, kills and position are only reported for players who are online.
        Playtime and last seen come from a separate endpoint that remembers everyone.
      </p>

      <LandClaimsDialog open={showClaims} onClose={() => setShowClaims(false)} />
      <KickAllDialog open={kickingAll} onClose={() => setKickingAll(false)} />

      <PlayerActionDialog
        pending={pending}
        players={players}
        onClose={() => setPending(null)}
        onRun={(done, run) => {
          setPending(null);
          action.mutate(run, {
            onSuccess: (result) => toast.success(done, { description: result.command }),
            onError: (error) =>
              toast.error("That did not work", {
                description: error instanceof ApiError ? error.message : "The action failed.",
              }),
          });
        }}
      />
    </div>
  );
}

/** Who owns keystones, and how much of the map they are holding. */
function LandClaimsDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  const serverId = useServerId();
  const { data, isLoading } = useQuery({
    queryKey: ["land-claims", serverId],
    queryFn: () => api.landClaims(serverId),
    enabled: open,
  });

  if (!open) return null;

  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>Land claims</DialogTitle>
          <DialogDescription>
            Claimed ground is the ground a chunk reset will not touch.
          </DialogDescription>
        </DialogHeader>
        {isLoading ? (
          <Skeleton className="h-40 w-full rounded-none" />
        ) : (
          <pre className="readout max-h-96 overflow-auto border border-border p-3 text-xs leading-relaxed whitespace-pre-wrap text-bone-dim">
            {data?.text.trim() || "The server said nothing."}
          </pre>
        )}
      </DialogContent>
    </Dialog>
  );
}

/**
 * Disconnects everybody, with a reason they get to read.
 *
 * The reason is the whole point of doing this from a panel rather than by
 * stopping the process: "restarting in five" lands in front of somebody mid
 * fight, and simply vanishing does not.
 */
function KickAllDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  const serverId = useServerId();
  const [reason, setReason] = useState("");

  const kick = useMutation({
    mutationFn: () => api.kickAll(serverId, reason.trim()),
    onSuccess: (result) => {
      toast.success("Everyone kicked", { description: result.command });
      onClose();
    },
    onError: (error) =>
      toast.error("That did not work", {
        description: error instanceof ApiError ? error.message : "The action failed.",
      }),
  });

  if (!open) return null;

  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Kick everyone</DialogTitle>
          <DialogDescription>
            Everybody online is disconnected. They can rejoin straight away.
          </DialogDescription>
        </DialogHeader>
        <Input
          value={reason}
          onChange={(e) => setReason(e.target.value)}
          placeholder="Reason, shown to each player"
          maxLength={200}
        />
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>
            Cancel
          </Button>
          <Button disabled={kick.isPending} onClick={() => kick.mutate()}>
            Kick everyone
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

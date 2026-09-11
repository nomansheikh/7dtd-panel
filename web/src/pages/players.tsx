import { useMemo, useState } from "react";
import {
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
import { MoreHorizontal } from "lucide-react";
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
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { PlayerActionDialog, type PendingAction } from "@/components/player-actions";
import { usePlayerAction, usePlayers } from "@/hooks/use-players";
import { ApiError, type Player } from "@/lib/api";
import { formatAge, formatUptime } from "@/lib/format";

function lastSeen(player: Player): string {
  if (player.online) return "now";
  if (!player.lastOnline) return "never";
  return formatAge((Date.now() - Date.parse(player.lastOnline)) / 1000);
}

// react-table v9 is feature-based: only the features declared here are wired
// into the table, which keeps the bundle to what is actually used.
const features = tableFeatures({ rowSortingFeature, globalFilteringFeature });

export function PlayersPage() {
  const { data, isLoading } = usePlayers();
  const action = usePlayerAction();
  const [filter, setFilter] = useState("");
  const [sorting, setSorting] = useState<SortingState>([]);
  const [pending, setPending] = useState<PendingAction | null>(null);

  const players = useMemo(() => data?.players ?? [], [data]);

  const columns = useMemo<ColumnDef<typeof features, Player, unknown>[]>(
    () => [
      {
        accessorKey: "name",
        header: "Player",
        cell: ({ row }) => (
          <div className="flex items-center gap-2">
            <span
              aria-hidden
              className={
                row.original.online
                  ? "size-2 shrink-0 rounded-full bg-status-online"
                  : "size-2 shrink-0 rounded-full bg-status-unknown"
              }
            />
            <span className="font-medium">{row.original.name}</span>
            {row.original.banned && <Badge variant="destructive">banned</Badge>}
          </div>
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
      {
        id: "actions",
        header: "",
        cell: ({ row }) => <PlayerActions player={row.original} onPick={setPending} />,
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
    _rowModels: {
      sortedRowModel: createSortedRowModel(sortFns),
      filteredRowModel: createFilteredRowModel(filterFns),
    },
  });

  const online = players.filter((p) => p.online).length;

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center gap-3">
        <h1 className="text-xl font-semibold">Players</h1>
        <span className="text-sm text-muted-foreground">
          {online} online · {players.length} known
        </span>
        <Input
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
          placeholder="Filter by name"
          className="ml-auto max-w-xs"
        />
      </div>

      {isLoading && !data ? (
        <Skeleton className="h-64 w-full" />
      ) : players.length === 0 ? (
        <p className="rounded-md border border-border bg-card p-6 text-sm text-muted-foreground">
          Nobody has joined this server yet. Players appear here as soon as they connect, and stay
          listed after they leave.
        </p>
      ) : (
        <div className="overflow-x-auto rounded-md border border-border bg-card">
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

      <PlayerActionDialog
        pending={pending}
        players={players}
        onClose={() => setPending(null)}
        onRun={(run) => {
          setPending(null);
          action.mutate(run, {
            onSuccess: (result) =>
              toast.success(result.result.trim() || "Done", {
                description: result.command,
              }),
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

function PlayerActions({
  player,
  onPick,
}: {
  player: Player;
  onPick: (action: PendingAction) => void;
}) {
  // Everything except ban needs the player online: the commands address them by
  // entity id, and an offline player has none.
  const offline = !player.online;

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="sm" aria-label={`Actions for ${player.name}`}>
          <MoreHorizontal />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem disabled={offline} onSelect={() => onPick({ kind: "teleport", player })}>
          Teleport
        </DropdownMenuItem>
        <DropdownMenuItem disabled={offline} onSelect={() => onPick({ kind: "give", player })}>
          Give item
        </DropdownMenuItem>
        <DropdownMenuItem disabled={offline} onSelect={() => onPick({ kind: "xp", player })}>
          Give XP
        </DropdownMenuItem>
        <DropdownMenuItem disabled={offline} onSelect={() => onPick({ kind: "buff", player })}>
          Buff or debuff
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem disabled={offline} onSelect={() => onPick({ kind: "kill", player })}>
          Kill
        </DropdownMenuItem>
        <DropdownMenuItem disabled={offline} onSelect={() => onPick({ kind: "kick", player })}>
          Kick
        </DropdownMenuItem>
        {player.banned ? (
          <DropdownMenuItem onSelect={() => onPick({ kind: "unban", player })}>
            Lift ban
          </DropdownMenuItem>
        ) : (
          <DropdownMenuItem variant="destructive" onSelect={() => onPick({ kind: "ban", player })}>
            Ban
          </DropdownMenuItem>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

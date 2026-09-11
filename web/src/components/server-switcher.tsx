import { Check, ChevronsUpDown, Server } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useServers } from "@/hooks/use-servers";
import { cn } from "@/lib/utils";
import type { ServerStatus } from "@/lib/api";

const DOT: Record<ServerStatus, string> = {
  online: "bg-status-online",
  degraded: "bg-status-degraded",
  offline: "bg-status-offline",
  unknown: "bg-status-unknown",
};

/**
 * Switches which game server the panel is acting on.
 *
 * With one server configured this collapses to a plain label: a dropdown with a
 * single choice is just noise.
 */
export function ServerSwitcher() {
  const { servers, current, currentId, select } = useServers();

  if (servers.length === 0) {
    return null;
  }

  if (servers.length === 1) {
    return (
      <span className="flex items-center gap-2 text-sm text-muted-foreground">
        <Server className="size-4" />
        {current?.name ?? currentId}
      </span>
    );
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="outline" size="sm" className="gap-2">
          <span
            aria-hidden
            className={cn("size-2 rounded-full", DOT[current?.status ?? "unknown"])}
          />
          {current?.name ?? "Select a server"}
          <ChevronsUpDown className="size-3.5 opacity-60" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="w-64">
        <DropdownMenuLabel>Game servers</DropdownMenuLabel>
        {servers.map((server) => (
          <DropdownMenuItem
            key={server.id}
            onSelect={() => select(server.id)}
            className="flex items-center gap-2"
          >
            <span aria-hidden className={cn("size-2 shrink-0 rounded-full", DOT[server.status])} />
            <span className="flex-1 truncate">{server.name}</span>
            {/* Player counts make the switcher useful at a glance rather than
                just a list of names. */}
            {server.status !== "unknown" && (
              <span className="text-xs text-muted-foreground">
                {server.players}/{server.maxPlayers || "?"}
              </span>
            )}
            {server.id === currentId && <Check className="size-4" />}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

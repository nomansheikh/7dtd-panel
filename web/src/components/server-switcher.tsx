import { Check, ChevronsUpDown, Plus } from "lucide-react";
import { Link } from "react-router-dom";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { SidebarMenu, SidebarMenuButton, SidebarMenuItem } from "@/components/ui/sidebar";
import { StatusDot } from "@/components/connection-status";
import { useServers } from "@/hooks/use-servers";

/**
 * Switches which game server the panel is acting on, and is the way to a
 * second one.
 *
 * It used to collapse to a plain label when only one server was configured, on
 * the reasoning that a dropdown with a single choice is noise. That made it a
 * dead end: having added the one server the first-run page asks for, there was
 * nowhere in the interface that offered another. A menu with one server and a
 * way to add the next is worth the click.
 */
export function ServerSwitcher() {
  const { servers, current, currentId, select } = useServers();

  if (servers.length === 0) {
    return null;
  }

  const label = current?.name ?? currentId;

  return (
    <SidebarMenu>
      <SidebarMenuItem>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <SidebarMenuButton size="lg" tooltip={label}>
              <StatusDot status={current?.status ?? "unknown"} />
              <div className="grid flex-1 text-left leading-tight">
                <span className="stencil">Game server</span>
                <span className="mt-1 truncate text-sm font-medium text-foreground">{label}</span>
              </div>
              <ChevronsUpDown className="ml-auto size-3.5 opacity-50" />
            </SidebarMenuButton>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="start" side="right" className="w-64">
            <DropdownMenuLabel className="stencil">Game servers</DropdownMenuLabel>
            {servers.map((server) => (
              <DropdownMenuItem
                key={server.id}
                onSelect={() => select(server.id)}
                className="flex items-center gap-2"
              >
                <StatusDot status={server.status} />
                <span className="flex-1 truncate">{server.name}</span>
                {/* Player counts make the switcher useful at a glance rather
                    than just a list of names. */}
                {server.status !== "unknown" && (
                  <span className="readout text-xs text-bone-faint">
                    {server.players}/{server.maxPlayers || "?"}
                  </span>
                )}
                {server.id === currentId && <Check className="size-4" />}
              </DropdownMenuItem>
            ))}
            <DropdownMenuSeparator />
            <DropdownMenuItem asChild>
              <Link to="/servers" className="gap-2">
                <Plus className="size-3.5 text-bone-faint" />
                <span className="text-sm">Add or manage servers</span>
              </Link>
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </SidebarMenuItem>
    </SidebarMenu>
  );
}

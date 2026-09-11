import { Check, ChevronsUpDown, HardDrive } from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { SidebarMenu, SidebarMenuButton, SidebarMenuItem } from "@/components/ui/sidebar";
import { StatusDot } from "@/components/connection-status";
import { useServers } from "@/hooks/use-servers";

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

  const label = current?.name ?? currentId;

  // One server is a label, not a control. Rendering it as a disabled button
  // left something in the tab order that takes focus and then does nothing.
  if (servers.length === 1) {
    return (
      <div className="flex items-center gap-2 px-2 py-1.5" title={label}>
        <HardDrive className="size-4 shrink-0 text-bone-faint" />
        <div className="grid flex-1 leading-tight group-data-[collapsible=icon]:hidden">
          <span className="stencil">Game server</span>
          <span className="mt-1 truncate text-sm font-medium text-foreground">{label}</span>
        </div>
      </div>
    );
  }

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
          </DropdownMenuContent>
        </DropdownMenu>
      </SidebarMenuItem>
    </SidebarMenu>
  );
}

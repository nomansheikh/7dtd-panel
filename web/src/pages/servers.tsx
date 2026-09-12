import { useState } from "react";
import { toast } from "sonner";
import { Pencil, Plus, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
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
import { ServerSheet } from "@/components/server-sheet";
import { useDeleteServer } from "@/hooks/use-server-admin";
import { useServers } from "@/hooks/use-servers";
import { type ServerSummary } from "@/lib/api";
import { cn } from "@/lib/utils";

/**
 * Every game server this panel manages.
 *
 * Reached from the switcher rather than the sidebar: it is somewhere you go
 * twice a year, and a permanent nav item for it would push the pages somebody
 * opens daily further down.
 */
export function ServersPage() {
  const { servers, currentId, select } = useServers();
  const [editing, setEditing] = useState<ServerSummary | null | undefined>(undefined);
  const [confirming, setConfirming] = useState<ServerSummary | null>(null);
  const remove = useDeleteServer();

  return (
    <div className="min-h-0 flex-1 overflow-y-auto">
      <div className="region-head">
        <span className="stencil">Game servers</span>
        <span className="readout text-xs text-bone-faint">{servers.length}</span>
        <Button
          variant="ghost"
          size="sm"
          className="ml-auto h-6 gap-1 px-2 text-xs"
          onClick={() => setEditing(null)}
        >
          <Plus className="size-3" />
          Add a server
        </Button>
      </div>

      <ul>
        {servers.map((server) => (
          <li
            key={server.id}
            className={cn(
              "group flex flex-wrap items-center gap-x-3 gap-y-2 border-b border-border px-4 py-3 md:px-6",
              server.id === currentId && "bg-accent/20",
            )}
          >
            <span
              className={cn(
                "size-1.5 shrink-0 rounded-full",
                server.status === "online" && "bg-emerald-500",
                server.status === "degraded" && "bg-ember",
                (server.status === "offline" || server.status === "unknown") && "bg-bone-faint",
              )}
              aria-hidden
            />
            <div className="min-w-0">
              <div className="flex flex-wrap items-baseline gap-2">
                <span className="text-sm text-bone">{server.name}</span>
                <span className="readout text-2xs text-bone-faint">{server.id}</span>
                {server.id === currentId && <span className="stencil text-ember">showing</span>}
              </div>
              <p className="readout mt-0.5 text-2xs text-bone-faint">
                {server.scheme ?? "http"}://{server.host}:{server.port}
                {server.world && <> · {server.world}</>}
                {server.version && <> · {server.version}</>}
              </p>
            </div>

            <div className="ml-auto flex items-center gap-1">
              {server.id !== currentId && (
                <Button
                  variant="ghost"
                  size="sm"
                  className="h-7 text-xs"
                  onClick={() => select(server.id)}
                >
                  Switch to it
                </Button>
              )}
              <span className="flex items-center gap-0.5 opacity-0 transition-opacity group-focus-within:opacity-100 group-hover:opacity-100">
                <Button
                  variant="ghost"
                  size="icon"
                  className="size-7 text-bone-faint"
                  aria-label={`Edit ${server.name}`}
                  onClick={() => setEditing(server)}
                >
                  <Pencil className="size-3" />
                </Button>
                <Button
                  variant="ghost"
                  size="icon"
                  className="size-7 text-bone-faint hover:text-crimson-lit"
                  aria-label={`Remove ${server.name}`}
                  onClick={() => setConfirming(server)}
                >
                  <Trash2 className="size-3" />
                </Button>
              </span>
            </div>
          </li>
        ))}
      </ul>

      <p className="max-w-prose px-4 py-4 text-2xs text-bone-faint md:px-6">
        Removing a server here only disconnects this panel from it. Nothing happens to the game
        server itself — but the chat commands and scheduled tasks you wrote for it are forgotten,
        since they mean nothing without it.
      </p>

      {editing !== undefined && (
        <ServerSheet editing={editing ?? undefined} onClose={() => setEditing(undefined)} />
      )}

      <AlertDialog open={confirming !== null} onOpenChange={(open) => !open && setConfirming(null)}>
        <AlertDialogContent>
          <AlertDialogTitle>Remove {confirming?.name}?</AlertDialogTitle>
          <AlertDialogHeader>
            <AlertDialogDescription>
              This panel stops managing it. The game server keeps running, but the chat commands and
              tasks written for it are forgotten.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Keep it</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => {
                if (!confirming) return;
                const name = confirming.name;
                remove.mutate(confirming.id, {
                  onSuccess: () => toast.success(`${name} removed`),
                  onError: (err) => toast.error("Not removed", { description: err.message }),
                });
                setConfirming(null);
              }}
            >
              Remove it
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}

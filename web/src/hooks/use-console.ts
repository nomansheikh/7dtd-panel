import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, type CommandInfo, type HistoryEntry } from "@/lib/api";
import { useServerId } from "@/hooks/use-servers";

/**
 * The selected server's own command catalogue.
 *
 * Cached hard: it is a couple of hundred entries that only change when the
 * server's mods do, and the panel already caches it server-side too.
 */
export function useCommands() {
  const serverId = useServerId();
  return useQuery<{ commands: CommandInfo[]; fetchedAt: string }>({
    queryKey: ["console", "commands", serverId],
    queryFn: () => api.commands(serverId),
    enabled: serverId !== "",
    staleTime: 30 * 60 * 1000,
    retry: 1,
  });
}

export function useHistory() {
  const serverId = useServerId();
  return useQuery<{ history: HistoryEntry[] }>({
    queryKey: ["console", "history", serverId],
    queryFn: () => api.history(serverId, 100),
    enabled: serverId !== "",
    retry: 1,
  });
}

export function useExecute() {
  const serverId = useServerId();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (command: string) => api.execute(serverId, command),
    onSettled: () => {
      // A command may have changed the world, and it always changes history.
      void queryClient.invalidateQueries({ queryKey: ["console", "history", serverId] });
      void queryClient.invalidateQueries({ queryKey: ["dashboard", serverId] });
    },
  });
}

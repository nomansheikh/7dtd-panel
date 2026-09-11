import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, type CommandInfo, type HistoryEntry } from "@/lib/api";

/**
 * The server's own command catalogue.
 *
 * Cached hard: it is ~200 entries that only change when the server's mods do,
 * and the panel already caches it server-side too.
 */
export function useCommands() {
  return useQuery<{ commands: CommandInfo[]; fetchedAt: string }>({
    queryKey: ["console", "commands"],
    queryFn: api.commands,
    staleTime: 30 * 60 * 1000,
    retry: 1,
  });
}

export function useHistory() {
  return useQuery<{ history: HistoryEntry[] }>({
    queryKey: ["console", "history"],
    queryFn: () => api.history(100),
    retry: 1,
  });
}

export function useExecute() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (command: string) => api.execute(command),
    onSettled: () => {
      // A command may have changed the world, and it always changes history.
      void queryClient.invalidateQueries({ queryKey: ["console", "history"] });
      void queryClient.invalidateQueries({ queryKey: ["dashboard"] });
    },
  });
}

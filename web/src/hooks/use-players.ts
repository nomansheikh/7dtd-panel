import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, type ActionResult, type Player } from "@/lib/api";
import { useServerId } from "@/hooks/use-servers";

/**
 * The player list for the selected server.
 *
 * Polled, because the game server has no push for player state: positions,
 * health and who is online all change constantly. placeholderData keeps the
 * table populated across a failed refetch for the same reason the dashboard
 * does.
 */
export function usePlayers() {
  const serverId = useServerId();
  return useQuery<{ players: Player[] }>({
    queryKey: ["players", serverId],
    queryFn: () => api.players(serverId),
    enabled: serverId !== "",
    refetchInterval: 10_000,
    placeholderData: (previous) => previous,
    retry: 1,
  });
}

/** Runs a player action and refreshes everything it could have changed. */
export function usePlayerAction() {
  const serverId = useServerId();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (run: () => Promise<ActionResult>) => run(),
    onSettled: () => {
      void queryClient.invalidateQueries({ queryKey: ["players", serverId] });
      void queryClient.invalidateQueries({ queryKey: ["dashboard", serverId] });
      void queryClient.invalidateQueries({
        queryKey: ["console", "history", serverId],
      });
    },
  });
}

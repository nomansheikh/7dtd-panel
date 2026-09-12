import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, type PowerStatus } from "@/lib/api";
import { useServerId } from "@/hooks/use-servers";

/**
 * The stop sequence, which runs on the panel rather than in this tab.
 *
 * Polled hard while something is happening and barely at all when it is not:
 * a countdown is the one screen somebody actually watches, and the rest of the
 * time there is nothing to see.
 */
export function usePower() {
  const serverId = useServerId();
  return useQuery({
    queryKey: ["power", serverId],
    queryFn: () => api.power(serverId),
    enabled: serverId !== "",
    refetchInterval: (query) => {
      const phase = query.state.data?.phase;
      const busy =
        phase === "countdown" || phase === "saving" || phase === "stopping" || phase === "watching";
      return busy ? 2000 : 30_000;
    },
  });
}

export function useStartPower() {
  const serverId = useServerId();
  const queryClient = useQueryClient();
  return useMutation<
    PowerStatus,
    Error,
    { intent: "restart" | "stop"; minutes: number; reason: string }
  >({
    mutationFn: (body) => api.startPower(serverId, body),
    onSuccess: (status) => queryClient.setQueryData(["power", serverId], status),
  });
}

export function useCancelPower() {
  const serverId = useServerId();
  const queryClient = useQueryClient();
  return useMutation<PowerStatus, Error, void>({
    mutationFn: () => api.cancelPower(serverId),
    onSuccess: (status) => queryClient.setQueryData(["power", serverId], status),
  });
}

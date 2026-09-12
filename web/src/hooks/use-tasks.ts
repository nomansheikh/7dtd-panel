import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, type TaskInput } from "@/lib/api";
import { useServerId } from "@/hooks/use-servers";

/**
 * What the panel does on its own, and what it has done lately.
 *
 * Polled, unlike the chat commands: the run history changes without anybody
 * touching this page, which is the entire point of it.
 */
export function useTasks() {
  const serverId = useServerId();
  return useQuery({
    queryKey: ["tasks", serverId],
    queryFn: () => api.tasks(serverId),
    enabled: serverId !== "",
    refetchInterval: 30_000,
    staleTime: 10_000,
  });
}

export function useSaveTask() {
  const serverId = useServerId();
  const queryClient = useQueryClient();
  return useMutation<{ ok: boolean }, Error, { name: string } & TaskInput>({
    mutationFn: ({ name, ...body }) => api.saveTask(serverId, name, body),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["tasks", serverId] });
    },
  });
}

export function useDeleteTask() {
  const serverId = useServerId();
  const queryClient = useQueryClient();
  return useMutation<{ ok: boolean }, Error, string>({
    mutationFn: (name) => api.deleteTask(serverId, name),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["tasks", serverId] });
    },
  });
}

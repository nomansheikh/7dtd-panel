import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, type Settings, type SettingUpdate } from "@/lib/api";
import { useServerId } from "@/hooks/use-servers";

/**
 * Every setting the server knows about, grouped and labelled.
 *
 * Not polled. The list is nearly three hundred entries and two round trips to
 * the game server, and nothing changes it except this page.
 */
export function useSettings() {
  const serverId = useServerId();
  return useQuery<Settings>({
    queryKey: ["settings", serverId],
    queryFn: () => api.settings(serverId),
    enabled: serverId !== "",
    staleTime: 30_000,
    retry: 1,
  });
}

/**
 * Changes one setting.
 *
 * The reply carries the value the server reports afterwards, which is not
 * always the one that was sent, so the row is updated from that rather than
 * from the request. The whole list is refetched too, because a preference can
 * move others with it.
 */
export function useUpdateSetting() {
  const serverId = useServerId();
  const queryClient = useQueryClient();
  return useMutation<SettingUpdate, Error, { name: string; value: string }>({
    mutationFn: ({ name, value }) => api.updateSetting(serverId, name, value),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["settings", serverId] });
      void queryClient.invalidateQueries({
        queryKey: ["console", "history", serverId],
      });
    },
  });
}

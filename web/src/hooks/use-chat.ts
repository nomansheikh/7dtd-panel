import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, type ChatAudience, type ChatCommand, type Kit, type KitItem } from "@/lib/api";
import { useServerId } from "@/hooks/use-servers";

/**
 * The chat bot's configuration.
 *
 * Not polled. Nothing changes it except this page, and the bot reads the
 * database directly rather than through here.
 */
export function useChatCommands() {
  const serverId = useServerId();
  return useQuery<{ prefix: string; commands: ChatCommand[] }>({
    queryKey: ["chat", "commands", serverId],
    queryFn: () => api.chatCommands(serverId),
    enabled: serverId !== "",
    staleTime: 30_000,
  });
}

export function useSaveChatCommand() {
  const serverId = useServerId();
  const queryClient = useQueryClient();
  return useMutation<
    { ok: boolean },
    Error,
    { name: string; enabled: boolean; audience: ChatAudience; cooldownSeconds: number }
  >({
    mutationFn: ({ name, ...body }) => api.saveChatCommand(serverId, name, body),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["chat", "commands", serverId] });
    },
  });
}

/**
 * The saved kits.
 *
 * Shared with the item picker, which reads the same key: a basket saved there
 * is a kit the bot can hand out, and they would be two different things if
 * they were two different caches.
 */
export function useKits() {
  const serverId = useServerId();
  return useQuery<Kit[]>({
    queryKey: ["chat", "kits", serverId],
    queryFn: async () => (await api.kits(serverId)).kits,
    enabled: serverId !== "",
    staleTime: 30_000,
  });
}

export function useSaveKit() {
  const serverId = useServerId();
  const queryClient = useQueryClient();
  return useMutation<Kit, Error, { name: string; items: KitItem[] }>({
    mutationFn: ({ name, items }) => api.saveKit(serverId, name, items),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["chat", "kits", serverId] });
    },
  });
}

export function useDeleteKit() {
  const serverId = useServerId();
  const queryClient = useQueryClient();
  return useMutation<{ ok: boolean }, Error, string>({
    mutationFn: (name) => api.deleteKit(serverId, name),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["chat", "kits", serverId] });
    },
  });
}

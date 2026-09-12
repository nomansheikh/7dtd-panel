import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api, type ServerInput, type ServerProbe } from "@/lib/api";

/**
 * Adding, editing and removing game servers.
 *
 * Every one of these invalidates the whole cache rather than just the server
 * list: a server appearing or vanishing changes what every other query means,
 * and a stale dashboard for a server that no longer exists is worse than a
 * moment of loading.
 */

export function useTestServer() {
  return useMutation<ServerProbe, Error, ServerInput>({
    mutationFn: (input) => api.testServer(input),
  });
}

export function useAddServer() {
  const queryClient = useQueryClient();
  return useMutation<{ id: string }, Error, ServerInput>({
    mutationFn: (input) => api.addServer(input),
    onSuccess: () => void queryClient.invalidateQueries(),
  });
}

export function useUpdateServer() {
  const queryClient = useQueryClient();
  return useMutation<{ id: string }, Error, { id: string } & ServerInput>({
    mutationFn: ({ id, ...input }) => api.updateServer(id, { id, ...input }),
    onSuccess: () => void queryClient.invalidateQueries(),
  });
}

export function useDeleteServer() {
  const queryClient = useQueryClient();
  return useMutation<{ ok: boolean }, Error, string>({
    mutationFn: (id) => api.deleteServer(id),
    onSuccess: () => void queryClient.invalidateQueries(),
  });
}

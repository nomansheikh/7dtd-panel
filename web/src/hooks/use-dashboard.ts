import { useQuery } from "@tanstack/react-query";
import { api, type Dashboard } from "@/lib/api";
import { useServerId } from "@/hooks/use-servers";

/**
 * Polls the dashboard for the currently selected server.
 *
 * placeholderData keeps the previous reading on screen while a refetch is in
 * flight or has failed, which is the client half of the same promise the
 * backend makes: a single failed poll must never blank the view. The server id
 * is in the query key, so switching servers shows a loading state rather than
 * the previous server's numbers.
 */
export function useDashboard() {
  const serverId = useServerId();
  return useQuery<Dashboard>({
    queryKey: ["dashboard", serverId],
    queryFn: () => api.dashboard(serverId),
    enabled: serverId !== "",
    refetchInterval: 5_000,
    refetchOnWindowFocus: true,
    placeholderData: (previous) => previous,
    retry: 1,
  });
}

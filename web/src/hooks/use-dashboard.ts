import { useQuery } from "@tanstack/react-query";
import { api, type Dashboard } from "@/lib/api";

/**
 * Polls the panel's dashboard endpoint.
 *
 * placeholderData keeps the previous reading on screen while a refetch is in
 * flight or has failed. That is the client half of the same promise the
 * backend makes: a single failed poll must never blank the view.
 */
export function useDashboard() {
  return useQuery<Dashboard>({
    queryKey: ["dashboard"],
    queryFn: api.dashboard,
    refetchInterval: 5_000,
    refetchOnWindowFocus: true,
    placeholderData: (previous) => previous,
    retry: 1,
  });
}

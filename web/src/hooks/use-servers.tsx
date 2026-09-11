import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import { api, type ServerSummary } from "@/lib/api";

const STORAGE_KEY = "sdtd-panel.server";

interface ServersValue {
  servers: ServerSummary[];
  /** The server every game call on screen is scoped to. */
  current: ServerSummary | null;
  currentId: string;
  select: (id: string) => void;
  loading: boolean;
}

const ServersContext = createContext<ServersValue | null>(null);

export function ServersProvider({ children }: { children: ReactNode }) {
  const { data, isLoading } = useQuery({
    queryKey: ["servers"],
    queryFn: api.servers,
    // Status is shown in the switcher, so keep it reasonably fresh.
    refetchInterval: 15_000,
    retry: 1,
  });

  const [selected, setSelected] = useState<string | null>(() => {
    try {
      return localStorage.getItem(STORAGE_KEY);
    } catch {
      // Private browsing and similar can throw here; a default is fine.
      return null;
    }
  });

  const servers = useMemo(() => data?.servers ?? [], [data]);

  // A remembered selection can name a server that has since been removed from
  // the configuration, so fall back rather than showing nothing.
  const currentId = useMemo(() => {
    if (selected && servers.some((s) => s.id === selected)) return selected;
    return data?.default ?? servers[0]?.id ?? "";
  }, [selected, servers, data]);

  useEffect(() => {
    if (!currentId) return;
    try {
      localStorage.setItem(STORAGE_KEY, currentId);
    } catch {
      // Not being able to remember the choice is not worth failing over.
    }
  }, [currentId]);

  const value = useMemo<ServersValue>(
    () => ({
      servers,
      current: servers.find((s) => s.id === currentId) ?? null,
      currentId,
      select: setSelected,
      loading: isLoading,
    }),
    [servers, currentId, isLoading],
  );

  return <ServersContext.Provider value={value}>{children}</ServersContext.Provider>;
}

export function useServers(): ServersValue {
  const value = useContext(ServersContext);
  if (!value) throw new Error("useServers must be used inside ServersProvider");
  return value;
}

/** The id of the server the UI is currently acting on. */
export function useServerId(): string {
  return useServers().currentId;
}

import { request } from "@/lib/api/request";
import type { ServerSummary, User } from "@/lib/api/types-panel";
import type { ServerInput, ServerProbe } from "@/lib/api/types-panel-features";

/** Signing in, and the servers the panel knows about — including how one comes
 * to exist at all. These are the only calls not scoped to a game server. */
export const panelApi = {
  login: (username: string, password: string) =>
    request<User>("/api/auth/login", {
      method: "POST",
      body: JSON.stringify({ username, password }),
    }),

  logout: () => request<void>("/api/auth/logout", { method: "POST" }),

  me: () => request<User>("/api/auth/me"),

  servers: () => request<{ servers: ServerSummary[]; default: string }>("/api/servers"),

  /** Does not save anything: it only says whether these details would work. */
  testServer: (input: ServerInput) =>
    request<ServerProbe>("/api/servers/test", {
      method: "POST",
      body: JSON.stringify(input),
    }),

  addServer: (input: ServerInput) =>
    request<{ id: string }>("/api/servers", {
      method: "POST",
      body: JSON.stringify(input),
    }),

  updateServer: (id: string, input: ServerInput) =>
    request<{ id: string }>(`/api/servers/${encodeURIComponent(id)}`, {
      method: "PUT",
      body: JSON.stringify(input),
    }),

  deleteServer: (id: string) =>
    request<{ ok: boolean }>(`/api/servers/${encodeURIComponent(id)}`, {
      method: "DELETE",
    }),
};

import { request, forServer } from "@/lib/api/request";
import type { ActionResult } from "@/lib/api/types-game";

/** Who is allowed on, and who is allowed to do what: admins, the whitelist,
 * bans and land claims. A ban takes a platform id rather than an entity id,
 * since it is the one action that must work for somebody already gone. */
export const accessApi = {
  access: (serverId: string) =>
    request<{ admins: string; whitelist: string }>(forServer(serverId, "/access")),

  /** Nought is full access; anybody without an entry sits at a thousand. */
  setAdmin: (serverId: string, platformUserId: string, level: number) =>
    request<ActionResult>(forServer(serverId, "/access/admin"), {
      method: "POST",
      body: JSON.stringify({ platformUserId, level }),
    }),

  removeAdmin: (serverId: string, platformUserId: string) =>
    request<ActionResult>(forServer(serverId, "/access/admin/remove"), {
      method: "POST",
      body: JSON.stringify({ platformUserId }),
    }),

  addToWhitelist: (serverId: string, platformUserId: string) =>
    request<ActionResult>(forServer(serverId, "/access/whitelist"), {
      method: "POST",
      body: JSON.stringify({ platformUserId }),
    }),

  removeFromWhitelist: (serverId: string, platformUserId: string) =>
    request<ActionResult>(forServer(serverId, "/access/whitelist/remove"), {
      method: "POST",
      body: JSON.stringify({ platformUserId }),
    }),
};

import { request, forServer } from "@/lib/api/request";
import type { ActionResult, BanUnit, Player } from "@/lib/api/types-game";
import type { SettingUpdate, Settings } from "@/lib/api/types-panel";

/** Players, and the settings that govern them. Every action names its target by
 * entity id, because a display name is chosen by the player. */
export const playersApi = {
  players: (serverId: string) => request<{ players: Player[] }>(forServer(serverId, "/players")),

  teleport: (
    serverId: string,
    entityId: number,
    to: { x: number; y: number; z: number } | { toEntityId: number },
  ) =>
    request<ActionResult>(forServer(serverId, `/players/${entityId}/teleport`), {
      method: "POST",
      body: JSON.stringify(to),
    }),

  giveItem: (serverId: string, entityId: number, item: string, count: number, quality: number) =>
    request<ActionResult>(forServer(serverId, `/players/${entityId}/give`), {
      method: "POST",
      body: JSON.stringify({ item, count, quality }),
    }),

  killPlayer: (serverId: string, entityId: number) =>
    request<ActionResult>(forServer(serverId, `/players/${entityId}/kill`), {
      method: "POST",
      body: "{}",
    }),

  kickPlayer: (serverId: string, entityId: number, reason: string) =>
    request<ActionResult>(forServer(serverId, `/players/${entityId}/kick`), {
      method: "POST",
      body: JSON.stringify({ reason }),
    }),

  buffPlayer: (serverId: string, entityId: number, buff: string, remove: boolean) =>
    request<ActionResult>(forServer(serverId, `/players/${entityId}/buff`), {
      method: "POST",
      body: JSON.stringify({ buff, remove }),
    }),

  giveXP: (serverId: string, entityId: number, amount: number) =>
    request<ActionResult>(forServer(serverId, `/players/${entityId}/xp`), {
      method: "POST",
      body: JSON.stringify({ amount }),
    }),

  banPlayer: (
    serverId: string,
    platformId: string,
    duration: number,
    unit: BanUnit,
    reason: string,
  ) =>
    request<ActionResult>(forServer(serverId, "/players/ban"), {
      method: "POST",
      body: JSON.stringify({ platformId, duration, unit, reason }),
    }),

  unbanPlayer: (serverId: string, platformId: string) =>
    request<ActionResult>(forServer(serverId, "/players/unban"), {
      method: "POST",
      body: JSON.stringify({ platformId, duration: 1, unit: "days", reason: "" }),
    }),

  settings: (serverId: string) => request<Settings>(forServer(serverId, "/settings")),

  updateSetting: (serverId: string, name: string, value: string) =>
    request<SettingUpdate>(forServer(serverId, `/settings/${encodeURIComponent(name)}`), {
      method: "PUT",
      body: JSON.stringify({ value }),
    }),
};

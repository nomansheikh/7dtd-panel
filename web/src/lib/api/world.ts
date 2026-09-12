import { request, forServer } from "@/lib/api/request";
import type {
  ActionResult,
  Buff,
  CommandInfo,
  EntityClass,
  ExecuteResult,
  GameItem,
  HistoryEntry,
  RawReply,
  Vitals,
  Weather,
  WeatherSetting,
} from "@/lib/api/types-game";
import type { Dashboard } from "@/lib/api/types-panel";

/** The world itself: its clock, its weather, what is spawned in it, and the
 * console that changes any of it. */
export const worldApi = {
  dashboard: (serverId: string) => request<Dashboard>(forServer(serverId, "/dashboard")),

  commands: (serverId: string) =>
    request<{ commands: CommandInfo[]; fetchedAt: string }>(
      forServer(serverId, "/console/commands"),
    ),

  execute: (serverId: string, command: string) =>
    request<ExecuteResult>(forServer(serverId, "/console/execute"), {
      method: "POST",
      body: JSON.stringify({ command }),
    }),

  history: (serverId: string, limit = 100) =>
    request<{ history: HistoryEntry[] }>(forServer(serverId, `/console/history?limit=${limit}`)),

  setTime: (serverId: string, day: number, hour: number, minute: number) =>
    request<ActionResult>(forServer(serverId, "/world/time"), {
      method: "POST",
      body: JSON.stringify({ day, hour, minute }),
    }),

  setWeather: (serverId: string, setting: WeatherSetting, value: number) =>
    request<ActionResult>(forServer(serverId, "/world/weather"), {
      method: "POST",
      body: JSON.stringify({ setting, value }),
    }),

  weather: (serverId: string) => request<Weather>(forServer(serverId, "/world/weather")),

  /** Screamers at a player. The bare command cannot be run remotely. */
  spawnScouts: (serverId: string, entityId: number) =>
    request<ActionResult>(forServer(serverId, "/world/scouts"), {
      method: "POST",
      body: JSON.stringify({ entityId }),
    }),

  /** The panel's own build and liveness, not the game's. */
  panelHealth: () => request<{ status: string; version: string }>("/api/health"),

  vitals: (serverId: string) => request<Vitals>(forServer(serverId, "/vitals")),

  /** Only saved once a player has been online about thirty seconds. */
  inventory: (serverId: string, entityId: number) =>
    request<RawReply>(forServer(serverId, `/players/${entityId}/inventory`)),

  unlockInventories: (serverId: string, entityId: number) =>
    request<ActionResult>(forServer(serverId, `/players/${entityId}/unlock`), {
      method: "POST",
      body: "{}",
    }),

  kickAll: (serverId: string, reason: string) =>
    request<ActionResult>(forServer(serverId, "/players/kickall"), {
      method: "POST",
      body: JSON.stringify({ reason }),
    }),

  landClaims: (serverId: string, player = "") =>
    request<RawReply>(
      forServer(serverId, `/land-claims${player ? `?player=${encodeURIComponent(player)}` : ""}`),
    ),

  removeLandClaims: (serverId: string, platformUserId: string) =>
    request<ActionResult>(forServer(serverId, "/land-claims/remove"), {
      method: "POST",
      body: JSON.stringify({ platformUserId }),
    }),

  setMaxPlayers: (serverId: string, count: number) =>
    request<ActionResult>(forServer(serverId, "/max-players"), {
      method: "POST",
      body: JSON.stringify({ count }),
    }),

  shutdown: (serverId: string) =>
    request<ActionResult>(forServer(serverId, "/shutdown"), { method: "POST", body: "{}" }),

  saveWorld: (serverId: string) =>
    request<ActionResult>(forServer(serverId, "/world/save"), { method: "POST", body: "{}" }),

  /** "" is hostiles only, "alive" adds animals, "all" is everything. */
  killAll: (serverId: string, scope: "" | "alive" | "all") =>
    request<ActionResult>(forServer(serverId, "/world/killall"), {
      method: "POST",
      body: JSON.stringify({ scope }),
    }),

  /** Permanently discards the saved data for every unprotected chunk. */
  resetChunks: (serverId: string) =>
    request<ActionResult>(forServer(serverId, "/world/reset-chunks"), {
      method: "POST",
      body: "{}",
    }),

  privateMessage: (serverId: string, entityId: number, message: string) =>
    request<ActionResult>(forServer(serverId, `/players/${entityId}/pm`), {
      method: "POST",
      body: JSON.stringify({ message }),
    }),

  airDrop: (serverId: string) =>
    request<ActionResult>(forServer(serverId, "/world/airdrop"), { method: "POST", body: "{}" }),

  storm: (serverId: string, biome: string, hours: number) =>
    request<ActionResult>(forServer(serverId, "/world/weather"), {
      method: "POST",
      body: JSON.stringify({ storm: true, biome, stormHours: hours }),
    }),

  resetWeather: (serverId: string) =>
    request<ActionResult>(forServer(serverId, "/world/weather"), {
      method: "POST",
      body: JSON.stringify({ defaults: true }),
    }),

  spawn: (serverId: string, entityClass: string, x: number, y: number, z: number, count: number) =>
    request<ActionResult>(forServer(serverId, "/world/spawn"), {
      method: "POST",
      body: JSON.stringify({ entityClass, x, y, z, count }),
    }),

  wanderingHorde: (serverId: string) =>
    request<ActionResult>(forServer(serverId, "/world/horde"), {
      method: "POST",
      body: "{}",
    }),

  say: (serverId: string, message: string) =>
    request<ActionResult>(forServer(serverId, "/world/say"), {
      method: "POST",
      body: JSON.stringify({ message }),
    }),

  /**
   * Every class the game will spawn from a command.
   *
   * Fetched whole rather than searched: it is a few hundred entries, cached on
   * the server and unchanging, so the picker can group and filter it without a
   * round trip per keystroke.
   */
  spawnableEntities: (serverId: string) =>
    request<{ entities: EntityClass[]; total: number }>(
      forServer(serverId, `/entities?q=&limit=600`),
    ),

  searchBuffs: (serverId: string, q: string) =>
    request<{ buffs: Buff[]; total: number }>(
      forServer(serverId, `/buffs?q=${encodeURIComponent(q)}`),
    ),

  /**
   * The whole item catalogue in one go.
   *
   * About fifteen hundred entries and 128 kB, static for the life of the
   * server, so the picker can browse and filter it without a round trip per
   * keystroke. Blocks are excluded: with them the list is twenty-six thousand
   * and it is not what anybody means by giving somebody something.
   */
  allItems: (serverId: string) =>
    request<{ items: GameItem[]; total: number }>(forServer(serverId, "/items?q=&limit=2000")),

  searchItems: (serverId: string, q: string) =>
    request<{ items: GameItem[]; total: number }>(
      forServer(serverId, `/items?q=${encodeURIComponent(q)}`),
    ),
};

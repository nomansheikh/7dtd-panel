/**
 * Typed client for the panel's own API.
 *
 * The browser only ever talks to the panel. It has no knowledge of the game
 * server's address and never sees its API token — every game call is proxied.
 */

export type ServerStatus = "unknown" | "online" | "degraded" | "offline";

export interface User {
  username: string;
}

/** One configured game server, as the switcher sees it. */
export interface ServerSummary {
  id: string;
  name: string;
  status: ServerStatus;
  version?: string;
  world?: string;
  players: number;
  maxPlayers: number;
  /** What a player types into the game's connect-to-IP box. */
  connect?: string;
}

export interface Dashboard {
  status: ServerStatus;
  /** True when the figures are cached from an earlier poll rather than fresh. */
  stale: boolean;
  ageSeconds: number;
  lastError?: string;
  server: {
    id: string;
    name: string;
    version: string;
    gameMode: string;
    /** What a player types into the game's connect-to-IP box. */
    connect?: string;
    description?: string;
    region?: string;
    /** How this panel reaches the server. Never contains credentials. */
    panelUrl: string;
  };
  players: { online: number; max: number };
  world: {
    name: string;
    day: number;
    hour: number;
    minute: number;
    hostiles: number;
    animals: number;
    /** How many of the 24 in-game hours are lit. Absent until first polled. */
    daylightHours?: number;
    /** Real minutes a whole game day takes. Absent until first polled. */
    dayMinutes?: number;
  };
  /** Null until the first sample; the API has no uptime field of its own. */
  uptime: { seconds: number } | null;
  bloodMoon: { active: boolean; nextDay: number; nextHour: number } | null;
}

/** One allowed value of a setting, labelled the way the game labels it. */
export interface SettingChoice {
  value: string;
  label: string;
}

export interface Setting {
  /** The raw preference name, e.g. BloodMoonFrequency. */
  name: string;
  label: string;
  description?: string;
  group: string;
  type: "bool" | "int" | "float" | "string";
  value: string | number | boolean | null;
  default: string | number | boolean | null;
  /** The game's word for the current value, e.g. "7 Days". */
  valueLabel?: string;
  defaultLabel?: string;
  changed: boolean;
  editable: boolean;
  readOnlyReason?: string;
  /** When present the control is a picker rather than a field. */
  choices?: SettingChoice[];
}

export interface SettingsGroup {
  name: string;
  settings: Setting[];
}

export interface SettingsSection {
  id: "world" | "server" | "client";
  title: string;
  description: string;
  groups: SettingsGroup[];
  total: number;
}

export interface Settings {
  /** The world's sandbox code, which reproduces these rules on a new world. */
  sandboxCode: string;
  sections: SettingsSection[];
}

export interface SettingUpdate {
  name: string;
  /** What the server reports after the change, read back rather than echoed. */
  value: string;
  command: string;
  /** Always false: the game keeps this in memory until it restarts. */
  persisted: boolean;
}

/** How much confirmation the panel demands before running a command. */
export type CommandTier = "normal" | "mutating" | "destructive";

export interface CommandInfo {
  name: string;
  aliases: string[];
  description: string;
  /** The game server's own usage text, absent for many commands. */
  help?: string;
  allowed: boolean;
  tier: CommandTier;
  /** True when PANEL_ALLOW_DESTRUCTIVE is off and this command is destructive. */
  blocked: boolean;
}

export interface ExecuteResult {
  command: string;
  parameters: string;
  result: string;
  tier: CommandTier;
  ranAt: string;
}

export interface HistoryEntry {
  id: number;
  command: string;
  succeeded: boolean;
  result?: string;
  error?: string;
  ranAt: string;
}

/** What the weather is doing in one biome. */
export interface BiomeWeather {
  biome: string;
  /** The server's own word: default, stormbuild or storm. */
  state: string;
  temperature: number;
  clouds: number;
  wind: number;
  fog: number;
  rain: number;
  snow: number;
}

export interface Weather {
  /**
   * What an admin has forced on top of the simulation.
   *
   * A cleared override and one deliberately set to zero both report 0; the
   * server gives no way to tell them apart.
   */
  overrides: {
    clouds: number;
    fog: number;
    rain: number;
    snow: number;
    temperature: number;
    wind: number;
  };
  biomes: BiomeWeather[];
}

/** The weather parameters the game's own weather command accepts. */
export type WeatherSetting = "Clouds" | "Rain" | "SnowFall" | "Wind" | "Temp" | "Fog";

export interface ActionResult {
  /** Echoed so the operator can see exactly what ran. */
  command: string;
  result: string;
  ranAt: string;
}

/** How hard the game server is working, and what it is running. */
export interface Vitals {
  fps: number;
  heapMb: number;
  maxHeapMb: number;
  rssMb: number;
  uptimeMinutes: number;
  chunks: number;
  players: number;
  zombies: number;
  entities: number;
  items: number;
  gameVersion: string;
  mods: { name: string; version: string }[];
}

/** A console reply handed back as the game wrote it. */
export interface RawReply {
  command: string;
  text: string;
}

export interface EntityClass {
  name: string;
  id: number;
  manualSpawnType: string;
}

export interface GameItem {
  name: string;
  localizedName: string;
  isBlock: boolean;
}

/** One buff or debuff the server will apply. */
export interface Buff {
  name: string;
  /** What the game calls it on screen; absent for internal buffs. */
  localizedName?: string;
}

export interface Position {
  x: number;
  y: number;
  z: number;
}

export interface Player {
  entityId: number;
  name: string;
  platformId: string;
  crossplatformId?: string;
  online: boolean;
  ping: number;
  /** Only known while online; /api/player is the sole source. */
  position?: Position;
  level: number;
  health: number;
  deaths: number;
  zombieKills: number;
  playerKills: number;
  playTimeSeconds: number;
  lastOnline?: string;
  banned: boolean;
  banReason?: string;
  banUntil?: string;
  ip?: string;
}

/** Duration units the game's ban command accepts. */
export type BanUnit = "minutes" | "hours" | "days" | "weeks" | "months" | "years";

export type EventKind = "log" | "chat" | "join" | "leave" | "death" | "status";

export interface PanelEvent {
  /** Panel-assigned and always increasing, so the UI can detect gaps. */
  seq: number;
  kind: EventKind;
  at: string;
  /** The game server's own log line number; absent for panel-generated events. */
  logId?: number;
  severity?: string;
  /** The part worth reading: what was said, not the log line around it. */
  message: string;
  player?: string;
  /** The chat channel, present only when it is not Global. */
  channel?: string;
  /** The server's original line, present only when message is a tidied form
      of it. Nothing is thrown away, it is just not what you read first. */
  raw?: string;
}

/** Who may run a chat command. The game's admin list decides who is an admin,
    not the panel's own logins: the people typing in chat have game identities. */
export type ChatAudience = "everyone" | "admins";

/** How much ceremony the panel demands of a console command. */
export type ChatTier = "normal" | "mutating" | "destructive";

/** One console line of a custom command, with what the panel knows about it. */
export interface ChatCommandLine {
  line: string;
  tier: ChatTier;
  /** Destructive, with PANEL_ALLOW_DESTRUCTIVE off. The operator's own switch. */
  blocked: boolean;
  /** Set when the line would be refused outright, so a typo shows at the keyboard. */
  problem?: string;
}

/** One command the bot answers: either built in, or one an admin wrote. */
export interface ChatCommand {
  name: string;
  /** Only a custom command can be edited or deleted. */
  kind: "builtin" | "custom";
  /** How a player types it, e.g. "!kit <name>". */
  usage: string;
  summary: string;
  /** True when it changes game state rather than reporting it. */
  acts: boolean;
  enabled: boolean;
  audience: ChatAudience;
  cooldownSeconds: number;
  /** What the player is told. Custom commands only. */
  reply?: string;
  commands?: ChatCommandLine[];
  /** The strongest tier of any of its lines. */
  tier?: ChatTier;
}

/** A token an admin can put in a reply or a command line. */
export interface ChatPlaceholder {
  token: string;
  means: string;
}

/** What an admin submits when writing or editing a command. */
export interface ChatCommandInput {
  enabled: boolean;
  audience: ChatAudience;
  cooldownSeconds: number;
  description?: string;
  reply?: string;
  commands?: string[];
}

/** One line of a kit: an item name, how many, and what quality. */
export interface KitItem {
  item: string;
  count: number;
  /** Below 1 means the item has no quality, or that the game should decide. */
  quality: number;
}

export interface Kit {
  name: string;
  items: KitItem[];
}

/** ApiError carries the panel's real message so the UI never has to invent one. */
export class ApiError extends Error {
  readonly status: number;
  readonly code?: string;

  constructor(status: number, message: string, code?: string, options?: { cause?: unknown }) {
    // The message is written for the operator; the cause keeps the underlying
    // failure available in the console for debugging.
    super(message, options);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
  }

  get isUnauthenticated() {
    return this.status === 401;
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  let response: Response;
  try {
    response = await fetch(path, {
      ...init,
      headers: {
        ...(init?.body ? { "Content-Type": "application/json" } : {}),
        ...init?.headers,
      },
      // The session lives in a cookie the browser cannot read.
      credentials: "same-origin",
    });
  } catch (cause) {
    // A failed fetch means the panel itself is unreachable, which is a
    // different problem from the game server being down, and worth saying so.
    throw new ApiError(0, "Could not reach the panel. Check that it is running.", "NETWORK", {
      cause,
    });
  }

  if (response.status === 204) {
    return undefined as T;
  }

  const text = await response.text();
  let body: unknown;
  try {
    body = text ? JSON.parse(text) : undefined;
  } catch {
    body = undefined;
  }

  if (!response.ok) {
    const err = body as { error?: string; code?: string } | undefined;
    throw new ApiError(
      response.status,
      err?.error ?? `The panel returned ${response.status}.`,
      err?.code,
    );
  }
  return body as T;
}

/** Builds the path for a call against one game server. */
function forServer(serverId: string, path: string): string {
  return `/api/servers/${encodeURIComponent(serverId)}${path}`;
}

export const api = {
  login: (username: string, password: string) =>
    request<User>("/api/auth/login", {
      method: "POST",
      body: JSON.stringify({ username, password }),
    }),

  logout: () => request<void>("/api/auth/logout", { method: "POST" }),

  me: () => request<User>("/api/auth/me"),

  servers: () => request<{ servers: ServerSummary[]; default: string }>("/api/servers"),

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

  chatCommands: (serverId: string) =>
    request<{
      prefix: string;
      commands: ChatCommand[];
      placeholders: ChatPlaceholder[];
      /** False when the operator turned destructive commands off panel-wide. */
      allowDestructive: boolean;
    }>(forServer(serverId, "/chat/commands")),

  saveChatCommand: (serverId: string, name: string, body: ChatCommandInput) =>
    request<{ ok: boolean }>(forServer(serverId, `/chat/commands/${encodeURIComponent(name)}`), {
      method: "PUT",
      body: JSON.stringify(body),
    }),

  /** Only a command an admin wrote; a built-in is switched off instead. */
  deleteChatCommand: (serverId: string, name: string) =>
    request<{ ok: boolean }>(forServer(serverId, `/chat/commands/${encodeURIComponent(name)}`), {
      method: "DELETE",
    }),

  /** Kits are panel-wide: the items belong to the game, not to one world. */
  kits: (serverId: string) => request<{ kits: Kit[] }>(forServer(serverId, "/chat/kits")),

  saveKit: (serverId: string, name: string, items: KitItem[]) =>
    request<Kit>(forServer(serverId, `/chat/kits/${encodeURIComponent(name)}`), {
      method: "PUT",
      body: JSON.stringify({ items }),
    }),

  deleteKit: (serverId: string, name: string) =>
    request<{ ok: boolean }>(forServer(serverId, `/chat/kits/${encodeURIComponent(name)}`), {
      method: "DELETE",
    }),

  /** The browser-facing event stream for one server. */
  eventsUrl: (serverId: string) => forServer(serverId, "/events"),
};

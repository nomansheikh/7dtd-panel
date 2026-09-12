/** Types for acting on a running world: the console, weather, players and the
 * event feed. */

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

/** Types for the panel itself: who is signed in, what servers exist, and what
 * the game server reports about its own configuration. */

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

  /** How the panel reaches it, for the edit form. Never includes the secret. */
  host?: string;
  port?: number;
  scheme?: "http" | "https";
  tokenName?: string;
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

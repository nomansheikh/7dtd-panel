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

export interface Dashboard {
  status: ServerStatus;
  /** True when the figures are cached from an earlier poll rather than fresh. */
  stale: boolean;
  ageSeconds: number;
  lastError?: string;
  server: { version: string; gameMode: string };
  players: { online: number; max: number };
  world: {
    name: string;
    day: number;
    hour: number;
    minute: number;
    hostiles: number;
    animals: number;
  };
  /** Null until the first sample; the API has no uptime field of its own. */
  uptime: { seconds: number } | null;
  bloodMoon: { active: boolean; nextDay: number; nextHour: number } | null;
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

/** The weather parameters the game's own weather command accepts. */
export type WeatherSetting = "Clouds" | "Rain" | "SnowFall" | "Wind" | "Temp" | "Fog";

export interface ActionResult {
  /** Echoed so the operator can see exactly what ran. */
  command: string;
  result: string;
  ranAt: string;
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

export type EventKind = "log" | "chat" | "join" | "leave" | "status";

export interface PanelEvent {
  /** Panel-assigned and always increasing, so the UI can detect gaps. */
  seq: number;
  kind: EventKind;
  at: string;
  /** The game server's own log line number; absent for panel-generated events. */
  logId?: number;
  severity?: string;
  message: string;
  player?: string;
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

export const api = {
  login: (username: string, password: string) =>
    request<User>("/api/auth/login", {
      method: "POST",
      body: JSON.stringify({ username, password }),
    }),

  logout: () => request<void>("/api/auth/logout", { method: "POST" }),

  me: () => request<User>("/api/auth/me"),

  dashboard: () => request<Dashboard>("/api/dashboard"),

  commands: () => request<{ commands: CommandInfo[]; fetchedAt: string }>("/api/console/commands"),

  execute: (command: string) =>
    request<ExecuteResult>("/api/console/execute", {
      method: "POST",
      body: JSON.stringify({ command }),
    }),

  history: (limit = 100) =>
    request<{ history: HistoryEntry[] }>(`/api/console/history?limit=${limit}`),

  setTime: (day: number, hour: number, minute: number) =>
    request<ActionResult>("/api/world/time", {
      method: "POST",
      body: JSON.stringify({ day, hour, minute }),
    }),

  setWeather: (setting: WeatherSetting, value: number) =>
    request<ActionResult>("/api/world/weather", {
      method: "POST",
      body: JSON.stringify({ setting, value }),
    }),

  resetWeather: () =>
    request<ActionResult>("/api/world/weather", {
      method: "POST",
      body: JSON.stringify({ defaults: true }),
    }),

  spawn: (entityClass: string, x: number, y: number, z: number, count: number) =>
    request<ActionResult>("/api/world/spawn", {
      method: "POST",
      body: JSON.stringify({ entityClass, x, y, z, count }),
    }),

  wanderingHorde: () => request<ActionResult>("/api/world/horde", { method: "POST", body: "{}" }),

  say: (message: string) =>
    request<ActionResult>("/api/world/say", {
      method: "POST",
      body: JSON.stringify({ message }),
    }),

  searchEntities: (q: string) =>
    request<{ entities: EntityClass[]; total: number }>(`/api/entities?q=${encodeURIComponent(q)}`),

  searchItems: (q: string) =>
    request<{ items: GameItem[]; total: number }>(`/api/items?q=${encodeURIComponent(q)}`),
};

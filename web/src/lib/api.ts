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
};

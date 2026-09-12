/**
 * The one way this app talks to its own backend.
 *
 * Every call goes through here so that a failed fetch, a 401 and a panel error
 * are told apart in one place rather than at each call site. The browser never
 * talks to a game server; the panel proxies all of that.
 */

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

export async function request<T>(path: string, init?: RequestInit): Promise<T> {
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
export function forServer(serverId: string, path: string): string {
  return `/api/servers/${encodeURIComponent(serverId)}${path}`;
}

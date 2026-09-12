import { featuresApi } from "@/lib/api/features";
import { accessApi } from "@/lib/api/access";
import { playersApi } from "@/lib/api/players";
import { worldApi } from "@/lib/api/world";
import { panelApi } from "@/lib/api/panel";

/**
 * Typed client for the panel's own API.
 *
 * The browser only ever talks to the panel. It has no knowledge of the game
 * server's address and never sees its API token — every game call is proxied.
 *
 * Split three ways by what a call acts on: the panel, a running world, and the
 * things the panel composes that the game has no notion of. They are merged
 * here so every call site still reads `api.something`.
 */
export const api = {
  ...panelApi,
  ...worldApi,
  ...playersApi,
  ...accessApi,
  ...featuresApi,
};

export { ApiError } from "@/lib/api/request";
export * from "@/lib/api/types-panel";
export * from "@/lib/api/types-game";
export * from "@/lib/api/types-panel-features";

import { forServer, request } from "@/lib/api/request";
import type { PowerStatus } from "@/lib/api/types-panel-features";

/**
 * Stopping a game server, and finding out whether it came back.
 *
 * The sequence runs on the panel, not in the browser: somebody who starts a
 * fifteen-minute countdown closes the tab and goes to bed.
 */
export const powerApi = {
  power: (serverId: string) => request<PowerStatus>(forServer(serverId, "/power")),

  /** Begins a countdown. Returns at once; the sequence runs on the panel. */
  startPower: (
    serverId: string,
    body: { intent: "restart" | "stop"; minutes: number; reason: string },
  ) =>
    request<PowerStatus>(forServer(serverId, "/power"), {
      method: "POST",
      body: JSON.stringify(body),
    }),

  cancelPower: (serverId: string) =>
    request<PowerStatus>(forServer(serverId, "/power/cancel"), { method: "POST", body: "{}" }),
};

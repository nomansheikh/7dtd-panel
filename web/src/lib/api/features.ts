import { request, forServer } from "@/lib/api/request";
import type {
  ChatCommand,
  ChatCommandInput,
  ChatPlaceholder,
  Kit,
  KitItem,
  Task,
  TaskInput,
  TaskRun,
} from "@/lib/api/types-panel-features";

/** What the panel composes on top of the game's API: chat commands it answers,
 * kits it hands out, and tasks it runs with nobody watching. */
export const featuresApi = {
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

  tasks: (serverId: string) =>
    request<{ tasks: Task[]; runs: TaskRun[]; allowDestructive: boolean }>(
      forServer(serverId, "/tasks"),
    ),

  saveTask: (serverId: string, name: string, body: TaskInput) =>
    request<{ ok: boolean }>(forServer(serverId, `/tasks/${encodeURIComponent(name)}`), {
      method: "PUT",
      body: JSON.stringify(body),
    }),

  deleteTask: (serverId: string, name: string) =>
    request<{ ok: boolean }>(forServer(serverId, `/tasks/${encodeURIComponent(name)}`), {
      method: "DELETE",
    }),

  /** The browser-facing event stream for one server. */
  eventsUrl: (serverId: string) => forServer(serverId, "/events"),
};

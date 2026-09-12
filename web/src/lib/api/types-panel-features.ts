/** Types for the things the panel composes that the game's API does not offer:
 * chat commands, kits, scheduled tasks, and adding a server. */

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

/** What an operator fills in to add or edit a game server. */
export interface ServerInput {
  id: string;
  name?: string;
  host: string;
  port?: number;
  scheme?: "http" | "https";
  tokenName: string;
  /** Blank on an edit keeps the stored one; the browser is never sent it. */
  tokenSecret?: string;
}

/** The answer to "would these details work?", asked before saving them. */
export interface ServerProbe {
  ok: boolean;
  /** Written for somebody mid-setup who does not know which half is wrong. */
  problem?: string;
  found?: {
    name?: string;
    world?: string;
    version?: string;
    players: number;
  };
}

/** What sets a task off. */
export type TaskTrigger =
  | "daily"
  | "every"
  | "gametime"
  | "bloodmoon"
  | "bloodmoonover"
  | "uptime"
  | "empty"
  | "join"
  | "leave"
  | "death";

/** One thing the panel does on its own. */
export interface Task {
  name: string;
  enabled: boolean;
  description?: string;
  trigger: TaskTrigger;
  /** The interval for "every", and how long before for "bloodmoon". */
  minutes?: number;
  /** "HH:MM" for "daily", in the panel's own timezone. */
  at?: string;
  commands: ChatCommandLine[];
  /** The strongest tier of any of its lines. */
  tier: ChatTier;
  lastRunAt?: string;
}

/** One record of a task going off. */
export interface TaskRun {
  name: string;
  ranAt: string;
  error?: string;
}

export interface TaskInput {
  enabled: boolean;
  description?: string;
  trigger: TaskTrigger;
  minutes?: number;
  at?: string;
  commands: string[];
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

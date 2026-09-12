import { type TaskTrigger } from "@/lib/api";

/**
 * What can set a task off, and the words for it.
 *
 * A table rather than a switch in the editor, because three different places
 * need the same words: the picker, the hint under it, and the sentence the list
 * prints beside each task.
 */

export const TRIGGERS: {
  value: TaskTrigger;
  label: string;
  hint: string;
  needs?: "time" | "interval" | "before" | "uptime";
}[] = [
  {
    value: "daily",
    label: "Every day at a time",
    hint: "Wall-clock time, in the panel's timezone. A nightly restart lives here.",
    needs: "time",
  },
  {
    value: "every",
    label: "On a repeat",
    hint: "Every so many real minutes, counted from the last time it ran.",
    needs: "interval",
  },
  {
    value: "gametime",
    label: "At an hour of the game's day",
    hint: "Every in-game day at this hour. On a world with thirty-minute days that comes round twice an hour, which is the point: dusk is dusk however fast the days run.",
    needs: "time",
  },
  {
    value: "bloodmoon",
    label: "Before the blood moon",
    hint: "Worked out from this world's own day length, so it lands at the right moment however fast the days run. Fires once per horde.",
    needs: "before",
  },
  {
    value: "bloodmoonover",
    label: "When the blood moon ends",
    hint: "Fires once, for a horde this panel saw begin. Starting the panel on a quiet afternoon does not count as having survived one.",
  },
  {
    value: "uptime",
    label: "Once the server has been up",
    hint: "Keyed to how long the server has actually been running rather than to a clock, so a restart is a restart whenever it last happened. Re-arms once it comes back.",
    needs: "uptime",
  },
  {
    value: "empty",
    label: "When the last player leaves",
    hint: "Fires once on emptying, not every time it is looked at. A save belongs here.",
  },
  {
    value: "join",
    label: "When a player joins",
    hint: "Runs for whoever connected. {player} and {entityid} stand for them.",
  },
  {
    value: "leave",
    label: "When a player leaves",
    hint: "Runs for whoever disconnected. Their entity id is gone by then, so {player} is the usable one.",
  },
  {
    value: "death",
    label: "When a player dies",
    hint: "Runs for whoever died. {player} and {entityid} stand for them.",
  },
];

export const INTERVALS = [
  { minutes: 15, label: "15 minutes" },
  { minutes: 30, label: "30 minutes" },
  { minutes: 60, label: "1 hour" },
  { minutes: 180, label: "3 hours" },
  { minutes: 360, label: "6 hours" },
  { minutes: 720, label: "12 hours" },
];

export const UPTIMES = [
  { minutes: 180, label: "3 hours" },
  { minutes: 360, label: "6 hours" },
  { minutes: 720, label: "12 hours" },
  { minutes: 1440, label: "a day" },
  { minutes: 4320, label: "3 days" },
];

export const WARNINGS = [
  { minutes: 5, label: "5 minutes before" },
  { minutes: 10, label: "10 minutes before" },
  { minutes: 15, label: "15 minutes before" },
  { minutes: 30, label: "30 minutes before" },
  { minutes: 60, label: "an hour before" },
];

import { type Task } from "@/lib/api";

/**
 * Turning a task's trigger into a sentence.
 *
 * The list, the row and the history all print the same facts, and a stretch of
 * minutes reads differently in each place if each works it out for itself.
 */

/** When it runs, in words. */
export function when(task: Task): string {
  const m = task.minutes ?? 0;
  switch (task.trigger) {
    case "daily":
      return `Every day at ${task.at}`;
    case "every":
      return `Every ${humanMinutes(m)}`;
    case "gametime":
      return `Every in-game day at ${task.at}`;
    case "bloodmoon":
      return `${humanMinutes(m)} before the blood moon`;
    case "bloodmoonover":
      return "When the blood moon ends";
    case "uptime":
      return `Once the server has been up ${humanMinutes(m)}`;
    case "empty":
      return "When the last player leaves";
    case "join":
      return "When a player joins";
    case "leave":
      return "When a player leaves";
    case "death":
      return "When a player dies";
  }
}

export function humanMinutes(m: number): string {
  if (m < 60) return `${m} minutes`;
  if (m < 1440) {
    const hours = m / 60;
    return hours === 1 ? "1 hour" : `${Number.isInteger(hours) ? hours : hours.toFixed(1)} hours`;
  }
  const days = m / 1440;
  return days === 1 ? "a day" : `${Number.isInteger(days) ? days : days.toFixed(1)} days`;
}

/** How long ago, roughly, for a line that is read at a glance. */
export function ago(iso: string): string {
  const seconds = Math.max(0, (Date.now() - new Date(iso).getTime()) / 1000);
  if (seconds < 90) return "just now";
  if (seconds < 3600) return `${Math.round(seconds / 60)}m ago`;
  if (seconds < 86400) return `${Math.round(seconds / 3600)}h ago`;
  return `${Math.round(seconds / 86400)}d ago`;
}

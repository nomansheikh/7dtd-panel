/** How many in-game days a blood moon cycle spans on a default server. */
export const BLOOD_MOON_CYCLE = 7;

/** Formats a duration for a readout: compact, and never jittering in width. */
export function formatUptime(seconds: number): string {
  const total = Math.max(0, Math.floor(seconds));
  const days = Math.floor(total / 86_400);
  const hours = Math.floor((total % 86_400) / 3600);
  const minutes = Math.floor((total % 3600) / 60);

  if (days > 0) return `${days}d ${hours}h`;
  if (hours > 0) return `${hours}h ${minutes}m`;
  return `${minutes}m`;
}

/** Formats the in-game clock as 24-hour time. */
export function formatGameClock(hour: number, minute: number): string {
  return `${String(hour).padStart(2, "0")}:${String(minute).padStart(2, "0")}`;
}

/** Describes how stale a reading is, in words a person would use. */
export function formatAge(seconds: number): string {
  if (seconds < 2) return "just now";
  if (seconds < 60) return `${Math.round(seconds)}s ago`;
  const minutes = Math.round(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  return `${Math.round(minutes / 60)}h ago`;
}

/**
 * Days remaining until the blood moon, and how far through the cycle we are.
 *
 * The game counts the blood moon as landing on the night of its day, so being
 * on that day already means it is tonight.
 */
export function bloodMoonProgress(currentDay: number, nextDay: number) {
  const daysAway = Math.max(0, nextDay - currentDay);
  const elapsed = Math.min(BLOOD_MOON_CYCLE, Math.max(0, BLOOD_MOON_CYCLE - daysAway));
  return { daysAway, elapsed, total: BLOOD_MOON_CYCLE };
}

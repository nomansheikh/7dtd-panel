import { useEffect } from "react";
import { useDashboard } from "@/hooks/use-dashboard";
import { bloodMoonProgress } from "@/lib/format";

/**
 * Publishes how close the blood moon is to CSS, as --moon on <html>.
 *
 * Kept here rather than passed down as props because it is read by the fixed
 * atmosphere layers, the sidebar, the rules between every panel and the glow
 * on the countdown — none of which are in the same part of the tree, and all
 * of which are answering the same question.
 *
 * --moon is 0 on the morning after a horde and 1 on the night of the next one.
 * --moon-heat is the same thing on a curve, and is what the colour actually
 * uses. Linear was wrong: by day five the panel had already gone as red as it
 * could get, which wasted the last two nights and made the interface shout
 * through the calm half of the week. Raised to a power, the first days stay
 * ash and the dread arrives where the game puts it.
 * --moon-night is 1 only while one is actually happening.
 */
export function useBloodMoon() {
  const { data } = useDashboard();

  const day = data?.world.day;
  const nextDay = data?.bloodMoon?.nextDay;
  const active = data?.bloodMoon?.active ?? false;

  useEffect(() => {
    const root = document.documentElement;

    // Before the first reading, and whenever the game server cannot tell us,
    // the panel sits at ash. Guessing at a countdown would be worse than
    // showing none: the whole point is that the colour can be trusted.
    if (day === undefined || nextDay === undefined) {
      root.style.setProperty("--moon", "0");
      root.style.setProperty("--moon-heat", "0");
      root.style.setProperty("--moon-night", "0");
      return;
    }

    const { elapsed, total } = bloodMoonProgress(day, nextDay);
    const moon = active ? 1 : elapsed / total;
    root.style.setProperty("--moon", moon.toFixed(3));
    root.style.setProperty("--moon-heat", Math.pow(moon, 2.6).toFixed(3));
    root.style.setProperty("--moon-night", active ? "1" : "0");
  }, [day, nextDay, active]);
}

import { useCallback, useEffect, useState } from "react";

/**
 * Dark, light, or whatever the machine says.
 *
 * The palette was written dark-only and says so in its own comment — this is a
 * thing you open at night next to a running game. The light set exists because
 * not everybody runs their panel at night, but it is a second reading of the
 * same colours rather than a generic white dashboard: the same warm greys and
 * the same crimson, turned over.
 */
export type Theme = "dark" | "light" | "system";

const KEY = "7dtd-panel.theme";

function systemPrefersDark(): boolean {
  return window.matchMedia?.("(prefers-color-scheme: dark)").matches ?? true;
}

/** Reads the stored choice, defaulting to the one the palette was built for. */
function stored(): Theme {
  const saved = localStorage.getItem(KEY);
  return saved === "light" || saved === "dark" || saved === "system" ? saved : "dark";
}

function apply(theme: Theme) {
  const dark = theme === "dark" || (theme === "system" && systemPrefersDark());
  const root = document.documentElement;
  root.classList.toggle("dark", dark);
  root.classList.toggle("light", !dark);
  // Tells the browser which way to render its own furniture: form controls,
  // scrollbars and the flash of background before the app paints.
  root.style.colorScheme = dark ? "dark" : "light";
}

export function useTheme() {
  const [theme, setThemeState] = useState<Theme>(stored);

  useEffect(() => {
    apply(theme);
    localStorage.setItem(KEY, theme);
  }, [theme]);

  // Following the system means following it as it changes, not only at load.
  useEffect(() => {
    if (theme !== "system") return;
    const media = window.matchMedia("(prefers-color-scheme: dark)");
    const onChange = () => apply("system");
    media.addEventListener("change", onChange);
    return () => media.removeEventListener("change", onChange);
  }, [theme]);

  const setTheme = useCallback((next: Theme) => setThemeState(next), []);
  return { theme, setTheme };
}

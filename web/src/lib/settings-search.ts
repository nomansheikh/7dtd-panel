import { type Setting, type SettingsSection } from "@/lib/api";

/**
 * Searching 426 preferences spread across four tabs.
 *
 * The search used to look only inside the tab you were already on, so typing
 * "map rendering" anywhere else answered "Nothing here matches" — while the
 * setting sat one tab over, editable. A search that lies about what exists is
 * worse than no search, because it ends the hunt.
 *
 * One matcher, used by the rows and by the tab counts, so a tab can never
 * promise a match the list then fails to show.
 */
export function matches(setting: Setting, needle: string, changedOnly: boolean): boolean {
  if (changedOnly && !setting.changed) return false;
  if (!needle) return true;
  return (
    setting.label.toLowerCase().includes(needle) || setting.name.toLowerCase().includes(needle)
  );
}

/** How many settings in a section match, for the count on its tab. */
export function countMatches(
  section: SettingsSection,
  needle: string,
  changedOnly: boolean,
): number {
  let n = 0;
  for (const group of section.groups) {
    for (const setting of group.settings) {
      if (matches(setting, needle, changedOnly)) n++;
    }
  }
  return n;
}

/** The other sections that do have matches, so an empty tab can point at them. */
export function sectionsWithMatches(
  sections: SettingsSection[],
  needle: string,
  changedOnly: boolean,
  except: string,
): SettingsSection[] {
  return sections.filter((s) => s.id !== except && countMatches(s, needle, changedOnly) > 0);
}

/**
 * A stable colour per player name.
 *
 * Twitch does this and it works: once you have seen a name in its colour a
 * couple of times you stop reading the name at all, you just see who is
 * talking. On a server with eight people in chat that is the difference
 * between a log and a conversation.
 *
 * The hues deliberately skip the 0–35 band, which is the blood moon's, and sit
 * at one lightness and chroma so no player's name shouts louder than another's.
 * They are light enough to clear 4.5:1 against the panel's near-black.
 */
const HUES = [45, 72, 100, 130, 160, 188, 212, 238, 268, 295, 318, 340];

/** How many distinct colours a server's chat can show before names repeat. */
export const USER_COLOURS = HUES.length;

/**
 * Picks a name's colour.
 *
 * FNV-1a rather than a sum of char codes: anagrams and near-identical names
 * ("Bob1", "Bob2") have to land far apart, which a sum does not manage.
 */
export function userColor(name: string): string {
  let hash = 0x811c9dc5;
  for (let i = 0; i < name.length; i++) {
    hash ^= name.charCodeAt(i);
    hash = Math.imul(hash, 0x01000193);
  }
  const hue = HUES[Math.abs(hash) % HUES.length];
  return `oklch(0.8 0.13 ${hue})`;
}

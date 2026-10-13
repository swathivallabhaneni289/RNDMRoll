/**
 * Small pure rules of the entry form and of Today's Posted and Skipped states (plan 03-02
 * Task 1, UI spec 3 and 4). No React, no imports, no clock: each file in `lib/entry` loads on
 * its own, so a node script can run it.
 *
 * Dates here are the server's civil date strings ("2026-10-08"). They are only ever compared
 * for equality as strings. The phone never works out a date, a weekday or whether a post is
 * late: the weekday in `nextSpinLine` is the server's `next_open_weekday`, printed as sent.
 */

/** Longest title and comment, in characters. The server checks the same numbers. */
export const TITLE_MAX = 80;
export const COMMENT_MAX = 140;

/** The title as it is sent: trimmed. */
export function cleanTitle(s: string): string {
  return s.trim();
}

export type MissingPart = 'photo' | 'title' | 'stars';

/** The order the footer line lists what is missing in. */
const PART_ORDER: readonly MissingPart[] = ['photo', 'title', 'stars'];

/**
 * What stops Save: `photo` when there is no photo at all, `title` when the title is empty after
 * trimming, `stars` when the rating is below 1 (0 means not rated and is never sent). The
 * result is in the order photo, title, stars; an empty list means Save may go.
 */
export function missingParts(a: { hasPhoto: boolean; title: string; stars: number }): MissingPart[] {
  const missing: MissingPart[] = [];
  if (!a.hasPhoto) missing.push('photo');
  if (cleanTitle(a.title).length === 0) missing.push('title');
  if (!(a.stars >= 1)) missing.push('stars');
  return missing;
}

/**
 * "Still needed: photo, title and stars." The list shrinks as parts are added ("Still needed:
 * title and stars.", "Still needed: stars."), is joined by commas and 'and', and is always in
 * the order photo, title, stars. Nothing missing gives an empty string (the line is not shown).
 */
export function stillNeededLine(missing: readonly MissingPart[]): string {
  const items = PART_ORDER.filter((part) => missing.includes(part));
  if (items.length === 0) return '';
  const last = items[items.length - 1];
  const list = items.length === 1 ? last : `${items.slice(0, -1).join(', ')} and ${last}`;
  return `Still needed: ${list}.`;
}

/**
 * The sentence under Skipped's heading. When the shown spin is today's (`spinLocalDate` equals
 * `localDate`, a plain string comparison) it names the server's weekday: "Your next spin opens
 * Friday at 8:00 PM." Otherwise (last night's spin in the late window), or when the server sent
 * no weekday, it leaves the weekday out. The space before "PM." is a no-break space (U+00A0) so
 * "PM." never stands alone on a line.
 */
export function nextSpinLine(
  spinLocalDate: string,
  localDate: string,
  nextOpenWeekday: string | null | undefined
): string {
  const hasWeekday = typeof nextOpenWeekday === 'string' && nextOpenWeekday.trim().length > 0;
  if (spinLocalDate === localDate && hasWeekday) {
    return `Your next spin opens ${nextOpenWeekday} at 8:00\u00A0PM.`;
  }
  return 'Your next spin opens at 8:00\u00A0PM.';
}

/** The small label above the word in Skipped: today's spin, or last night's in the late window. */
export function settledLabel(spinLocalDate: string, localDate: string): string {
  return spinLocalDate === localDate ? "Today's spin was" : "Last night's spin was";
}

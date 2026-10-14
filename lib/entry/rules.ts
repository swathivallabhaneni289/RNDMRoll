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

// ---- Plan 03-03: the entry page, edit and skip --------------------------------------------------

const ENTRY_ID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

/**
 * An entry id is a 36 character UUID. A route that is handed anything else shows 'This entry isn't
 * here any more.' and never sends the id to the server (the server answers not found for it too).
 */
export function isEntryId(value: unknown): value is string {
  return typeof value === 'string' && ENTRY_ID.test(value);
}

/**
 * The body of the skip confirmation. The first sentence names the person's count this week and is
 * left out when the shown spin is from the week before the week section's week (a plain string
 * compare of two server dates: a skip just after midnight on Monday for Sunday's spin).
 */
export function skipBody(spinLocalDate: string, weekStart: string, posted: number, goal: number): string {
  const rest = "A skipped day doesn't count toward it, and it isn't a miss. You can't post for this day afterwards.";
  if (spinLocalDate < weekStart) {
    return "A skipped day doesn't count toward your week, and it isn't a miss. You can't post for this day afterwards.";
  }
  return `You are at ${posted} of ${goal} this week. ${rest}`;
}

/** The fields an edit may send; `''` clears a reaction or a comment. */
export type EntryFieldPatch = { title?: string; stars?: number; reaction?: string; comment?: string };

/**
 * Only what the person changed. The title is compared and sent trimmed, the comment the same way;
 * a cleared reaction or comment is sent as the empty string. No key when nothing differs.
 */
export function changedFields(
  original: { title: string; stars: number; reaction: string | null; comment: string | null },
  current: { title: string; stars: number; reaction: string | null; comment: string }
): EntryFieldPatch {
  const patch: EntryFieldPatch = {};
  const title = cleanTitle(current.title);
  if (title !== cleanTitle(original.title)) patch.title = title;
  if (current.stars !== original.stars) patch.stars = current.stars;
  if (current.reaction !== original.reaction) patch.reaction = current.reaction ?? '';
  const comment = current.comment.trim();
  if (comment !== (original.comment ?? '').trim()) patch.comment = comment;
  return patch;
}

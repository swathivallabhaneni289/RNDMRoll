/**
 * The morning note (developer, 2026-10-28): when a spin was made and is not yet posted or skipped,
 * the phone says so two hours before a post would count as late. Today's `late_from` is noon the
 * day after the spin, so the note comes at 10 AM: "Your movie is waiting. Post it by noon to be on
 * time." Pure: no imports, so a node script can read it (scripts/check-push.ts).
 */

/** How long before `late_from` the note comes: 10 AM for a noon `late_from`. */
export const POST_NOTE_LEAD_MS = 2 * 60 * 60 * 1000;

/** The note's line. The category is in lower case, as on the Board ("Log your meal"). */
export function postNoteBody(categoryName: string): string {
  return `Your ${categoryName.toLowerCase()} is waiting. Post it by noon to be on time.`;
}

/**
 * When the note comes: two hours before `late_from`, as an absolute moment, so it follows the
 * server's noon whatever zone the phone is in. Null when there is no spin waiting for a post
 * (`late_from` is null) or that moment has already passed.
 */
export function postNoteDate(nowMs: number, lateFromMs: number | null): Date | null {
  if (lateFromMs === null || !Number.isFinite(lateFromMs)) return null;
  const at = lateFromMs - POST_NOTE_LEAD_MS;
  return at > nowMs ? new Date(at) : null;
}

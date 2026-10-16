import { monthDay, monthDayLong } from '@/components/profile/summaryText';
import { starsSpoken } from '@/lib/entry/stars';

/**
 * The words of the Diary's rows (04-UI-SPEC sections 3 and 8): pure functions from what the server
 * sent to what is printed and spoken. The only date work is the Profile page's fixed month table
 * ("2026-10-08" to "Oct 8"); the phone works out no date, weekday or number of its own.
 */

/** "Oct 8 · " before the category on a row (the category is drawn on its own so it can be cut). */
export function datePrefix(localDate: string): string {
  const date = monthDay(localDate);
  return date === '' ? '' : `${date} · `;
}

/** The stars figure, always one decimal as the picture writes it: 7 gives "3.5", 10 gives "5.0". */
export function starsFigure(stars: number): string {
  return (stars / 2).toFixed(1);
}

/** The note line in curly quotation marks, or null when the entry has none. */
export function quotedNote(comment: string | null): string | null {
  if (comment === null || comment.trim() === '') return null;
  return `“${comment.trim()}”`;
}

/** "Mushroom ramen. 4.5 out of 5 stars. Meal. October 8. Posted late. Note: The broth was worth it." */
export function rowSpoken(entry: {
  title: string;
  stars: number;
  category: { name: string };
  local_date: string;
  late: boolean;
  comment: string | null;
}): string {
  const parts = [`${entry.title}.`, `${starsSpoken(entry.stars)}.`, `${entry.category.name}.`];
  const date = monthDayLong(entry.local_date);
  if (date !== '') parts.push(`${date}.`);
  if (entry.late) parts.push('Posted late.');
  if (entry.comment !== null && entry.comment.trim() !== '') parts.push(`Note: ${entry.comment.trim()}.`);
  return parts.join(' ');
}

/** "Skipped. October 6. Meal." */
export function skippedSpoken(localDate: string, categoryName: string): string {
  const date = monthDayLong(localDate);
  return date === '' ? `Skipped. ${categoryName}.` : `Skipped. ${date}. ${categoryName}.`;
}

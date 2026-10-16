/**
 * The words of the Board, the post page and Notifications (Phase 5, with the follow wording of
 * 05-FOLLOW-OVERRIDES section 6). Pure: no React and no imports, so a node script can read it.
 * The phone works nothing out: every count, label and date comes from the server.
 */

export type DayLabel = 'today' | 'last_night';

/** The small capitals above the heading 'Board'. */
export function overline(dayLabel: DayLabel): string {
  return dayLabel === 'last_night' ? 'Last night' : 'Today';
}

/** "3 people you follow posted today." / "1 person you follow posted today." / none yet. */
export function postedLine(count: number, dayLabel: DayLabel): string {
  const when = dayLabel === 'last_night' ? 'last night' : 'today';
  if (count <= 0) return 'No one you follow has posted yet.';
  if (count === 1) return `1 person you follow posted ${when}.`;
  return `${count} people you follow posted ${when}.`;
}

export const LOCKED_BODY = 'Post your spin to see what they got and how they rated it.';

/** The locked card's button: "Log your meal", "Spin first", or none while nothing can be posted. */
export function lockedButton(action: 'log' | 'spin' | 'wait', category: string | null): string | null {
  if (action === 'log') return category ? `Log your ${category.toLowerCase()}` : 'Log your spin';
  if (action === 'spin') return 'Spin first';
  return null;
}

/** The quiet line under the locked card's text. The space before PM never breaks. */
export function lockedFootnote(action: 'log' | 'spin' | 'wait', dayLabel: DayLabel): string | null {
  if (action === 'wait') return 'Your next spin opens at 8:00 PM.';
  if (action === 'log' && dayLabel === 'last_night') return 'Posting it now would count as late.';
  return null;
}

/** After a skip: the one rule for everyone (a skip does not open the day). */
export function skippedLine(dayLabel: DayLabel): string {
  const when = dayLabel === 'last_night' ? 'last night' : 'today';
  return `You skipped ${when}. Posts from people you follow open after your next post.`;
}

export const NO_FOLLOWS_LINE = 'Follow people to see their spins here.';

/** The stars as a figure with one decimal: 7 gives "3.5", 10 gives "5.0". */
export function starsFigure(stars: number): string {
  return (Math.max(0, Math.min(10, stars)) / 2).toFixed(1);
}

/** "1 comment", "3 comments"; nothing at 0 or when unknown. */
export function commentCount(n: number | null): string {
  if (n === null || n <= 0) return '';
  return n === 1 ? '1 comment' : `${n} comments`;
}

/** What VoiceOver says for a card: "Aftersun. 5.0 out of 5 stars. Movie. By Maya." */
export function cardSpoken(a: { title: string; stars: number; category: string; by: string; late: boolean }): string {
  const late = a.late ? ' Posted late.' : '';
  return `${a.title}. ${starsFigure(a.stars)} out of 5 stars. ${a.category}. By ${a.by}.${late}`;
}

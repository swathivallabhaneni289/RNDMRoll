/**
 * The words of the Board, the post page, likes, reports and Notifications (Phase 5, with the follow
 * wording of 05-FOLLOW-OVERRIDES section 6). Pure: no React and no imports, so a node script can
 * read it. The phone works nothing out: every count, label and date comes from the server.
 */

/**
 * The small capitals above the heading 'Board'. The day in play runs from 8 PM to the next 8 PM, so
 * it is always today (developer, 2026-10-28), also after midnight.
 */
export const OVERLINE = 'Today';

/** "3 people you follow posted today." / "1 person you follow posted today." / none yet. */
export function postedLine(count: number): string {
  const when = 'today';
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

/**
 * The quiet line under the locked card's text. The space before PM never breaks. The late line
 * shows once a post made now counts as late (the server's `post_late`, from noon the day after
 * the spin), not from midnight. While waiting it names the time Spin's countdown counts to
 * (`opensAt`, "8:00 PM" when left out): after a big time zone change the next spin can open later
 * that evening, "Your next spin opens at 9:50 PM."
 */
export function lockedFootnote(
  action: 'log' | 'spin' | 'wait',
  postLate: boolean,
  opensAt: string | null = null
): string | null {
  if (action === 'wait') return `Your next spin opens at ${(opensAt ?? '8:00 PM').replace(' ', '\u00A0')}.`;
  if (action === 'log' && postLate) return 'Fashionably late is fine until 8:00 PM.';
  return null;
}

/** After a skip: the one rule for everyone (a skip does not open the day). */
export const SKIPPED_LINE = 'You skipped today. Posts from people you follow open after your next post.';

/** The empty Board before following anyone: a heading and one line on how the Board works. */
export const NO_FOLLOWS_TITLE = "See your friends' spins";
export const NO_FOLLOWS_LINE = 'Follow people you know. Their posts show here on the days you post too.';

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

/** The post page's one line when the post is not there for the viewer, and a comment's failure line then. */
export const POST_GONE_LINE = "This post isn't here any more.";

/**
 * The like (BRD-06, developer, 2026-11-24): a heart beside the comment mark. On someone else's post
 * a tap likes it and a second tap undoes it; on your own post it shows how many liked it and opens
 * the Likes list. What VoiceOver says for the heart, the Likes sheet's words, and the line when a
 * like would not save.
 */
export const LIKE_LABEL = 'Like';
export const LIKES_TITLE = 'Likes';
export const LIKES_CLOSE = 'Close likes';
export const NO_LIKES = 'No likes yet.';
export const LIKES_LOADING = 'Loading likes.';
export const LIKES_HINT = 'Opens who liked it.';
export const LIKE_FAILED = "Couldn't save that. Please try again.";

/** The bell's line after the person's first name, as every row names them: "Maya liked your post." */
export const LIKED_YOUR_POST = ' liked your post.';

/** A like count as a whole number, 0 when there is none or it is unknown. */
function wholeLikes(n: number): number {
  return Number.isFinite(n) && n > 0 ? Math.floor(n) : 0;
}

/** The number beside the heart on your own post: "3"; nothing at 0 or when unknown. */
export function likeCount(n: number): string {
  const count = wholeLikes(n);
  return count > 0 ? String(count) : '';
}

/** What VoiceOver says for the heart on your own post: "No likes yet", "1 like", "3 likes". */
export function likesSpoken(n: number): string {
  const count = wholeLikes(n);
  if (count === 0) return 'No likes yet';
  return count === 1 ? '1 like' : `${count} likes`;
}

/**
 * The line for a failed comment, reply, like or delete, by the server's error code (null when the
 * failure has none). A like that would not save always gets LIKE_FAILED, whatever the code. No
 * connection and too many tries keep the client's own words: socialErrorMessage in
 * lib/api/board.ts gives those first. A comment answered `not_found` means the post went away for
 * the viewer (deleted, archived, a block, or no longer followed): commenting being off is answered
 * before it, so nothing else gives it.
 */
export function commentFailureLine(code: string | null, what: 'comment' | 'reply' | 'like' | 'other'): string {
  if (what === 'like') return LIKE_FAILED;
  if (code === 'comments_off') return 'Commenting is off.';
  if (code === 'not_found' && what === 'reply') return "That comment isn't here any more.";
  if (code === 'not_found' && what === 'comment') return POST_GONE_LINE;
  if (what === 'comment') return "Couldn't post your comment. Please try again.";
  if (what === 'reply') return "Couldn't post your reply. Please try again.";
  return 'Something went wrong. Please try again.';
}

/**
 * The one line after a report of a post, a comment or a person (moderation, 2026-11-17), shown for
 * a moment over whatever page is showing (lib/report/notice.ts).
 */
export const REPORT_SENT = 'Report sent.';

/**
 * How a report ended, by the answer's status and error code (the three report flows ask this
 * through reportFailureLine in lib/api/reports.ts). A 404 is done, like a 200: the server answers a
 * repeat report of a post or comment like a miss, and a post, comment or person that went away is
 * gone for the reporter either way. No connection and too many tries keep the client's own words.
 */
export function reportOutcome(status: number, code: string | null): 'done' | 'client' | 'generic' {
  if (status === 404 || code === 'not_found') return 'done';
  if (status === 429 || code === 'rate_limited' || code === 'network_unavailable') return 'client';
  return 'generic';
}

/**
 * Under your own post or comment while 3 or more reports wait for the admin (the server's
 * `under_review`, sent only to you). Until the admin looks, no one else sees it.
 */
export const UNDER_REVIEW = 'Hidden while we take a look.';

/** The Spin page's posted row when an admin removed the day's post (Today's `entry_removed`). */
export const POST_REMOVED_LINE = 'We removed this post. The day still counts.';

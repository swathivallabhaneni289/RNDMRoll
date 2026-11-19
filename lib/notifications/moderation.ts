/**
 * The bell's items about your own post or comment under moderation: an admin's decision
 * (2026-11-17), or the reports that hid it while we take a look (2026-11-18). A removal names the
 * reason the admin gave (2026-11-18), which the server sends as a report reason's key. A removal
 * also opens a page to ask for a second look (developer, 2026-11-19: "they can like have like a
 * comment section, and they can upload their file or something, and the proofs"): why it should
 * come back and up to 3 photos, once per removal, inside its 30 days. The admin answers with Bring
 * it back (the back kinds) or Keep it removed (the kept kinds). None of these items has a person.
 * Pure: no imports, so a node script can check it (scripts/check-social.ts).
 */

/** The eight kinds, as the server names them. */
export const DECISION_KINDS = [
  'post_removed',
  'comment_removed',
  'post_back',
  'comment_back',
  'post_kept',
  'comment_kept',
  'post_hidden',
  'comment_hidden',
] as const;

export type DecisionKind = (typeof DECISION_KINDS)[number];

export function isDecisionKind(kind: string): kind is DecisionKind {
  return (DECISION_KINDS as readonly string[]).includes(kind);
}

/** The two removals: the kinds that can ask for a second look. */
export type RemovalKind = 'post_removed' | 'comment_removed';

export function isRemovalKind(kind: string): kind is RemovalKind {
  return kind === 'post_removed' || kind === 'comment_removed';
}

/**
 * A removal's reason in its sentence, by the report reason's key the server sends: the words the
 * report list shows (lib/api/person.ts), lowercased. Something else (`other`) has none here, so its
 * removal keeps the plain sentence.
 */
export const REASON_WORDS: Readonly<Record<string, string>> = {
  spam: 'spam',
  nudity: 'nudity or sexual content',
  harassment: 'bullying or harassment',
  hate: 'hate speech',
  violence: 'violence or threats',
  impersonation: 'pretending to be someone else',
};

/** The words of a reason, '' for Something else, none, or a key this app doesn't know. */
function reasonWords(reason: string): string {
  return Object.prototype.hasOwnProperty.call(REASON_WORDS, reason) ? REASON_WORDS[reason] : '';
}

/**
 * The row's sentence. Removed: the reason the admin gave (`reason`, a report reason's key), or for
 * Something else or none, that it broke the rules. Back: the admin brought it back, or dismissed
 * the reports that had hidden it. Kept: the admin looked again after a second look and kept it
 * removed. Hidden: 3 reports hid it until the admin looks. Only a removal reads `reason`.
 */
export function decisionLine(kind: DecisionKind, reason = ''): string {
  const words = reasonWords(reason);
  switch (kind) {
    case 'post_removed':
      return words ? `We removed your post for ${words}.` : 'We removed your post. It broke our rules.';
    case 'comment_removed':
      return words ? `We removed your comment for ${words}.` : 'We removed your comment. It broke our rules.';
    case 'post_back':
      return 'Sorry, we got that wrong. Your post is back.';
    case 'comment_back':
      return 'Sorry, we got that wrong. Your comment is back.';
    case 'post_kept':
      return 'We looked again. Your post stays removed.';
    case 'comment_kept':
      return 'We looked again. Your comment stays removed.';
    case 'post_hidden':
      return 'Your post is hidden while we take a look.';
    case 'comment_hidden':
      return 'Your comment is hidden while we take a look.';
  }
}

/** What the bell reads of a removal's second look (lib/api/notifications.ts). */
type SecondLookFields = { kind: string; removal: string; second_look: string };

/**
 * True while a removal can still ask for a second look: the server sends `second_look` '' until
 * one is asked ("asked", then "kept" or "restored"; "restored" also when a Bring it back came
 * first), and `removal`, the id the request names. Any other kind, any other state (one this app
 * doesn't know too) or no id cannot, and a tap on it does nothing.
 */
export function canAskSecondLook<T extends SecondLookFields>(item: T): item is T & { kind: RemovalKind } {
  return isRemovalKind(item.kind) && item.removal !== '' && item.second_look === '';
}

/** True on a removal whose second look is asked and not yet answered: its row says so. */
export function isSecondLookAsked(item: SecondLookFields): boolean {
  return isRemovalKind(item.kind) && item.second_look === 'asked';
}

/** Under a removal's line once its second look is asked, until the admin answers. */
export const SECOND_LOOK_ASKED = 'Second look asked.';

/** What a screen reader adds on a removal that opens the second look's page. */
export const SECOND_LOOK_HINT = 'Opens a page to ask for a second look.';

/** The second look's page (components/notifications/SecondLookScreen.tsx). */
export const SECOND_LOOK_TITLE = 'Ask for a second look';
export const SECOND_LOOK_LABEL = 'Tell us why it should come back';
export const SECOND_LOOK_PHOTOS = 'Add photos';
/** Over the page the bell shows again after a send (lib/report/notice.ts). */
export const SECOND_LOOK_SENT = "Sent. We'll take another look.";

/** The server's limits: 1 to 500 characters once trimmed, and up to 3 photos. */
export const SECOND_LOOK_MAX_CHARS = 500;
export const SECOND_LOOK_MAX_PHOTOS = 3;

/**
 * The item's line at the top of the page: "Your post:" or "Your comment:", then the post's title or
 * the comment's words, as the bell has them. A comment that was only a photo has no words, so its
 * line says so.
 */
export function secondLookItemLine(kind: RemovalKind, text: string): { label: string; words: string } {
  const post = kind === 'post_removed';
  return { label: post ? 'Your post:' : 'Your comment:', words: post || text.trim() !== '' ? text : 'a photo' };
}

const EMPTY_LINE = 'Tell us why first.';
const TOO_LONG_LINE = 'Keep it under 500 characters.';

/**
 * The text box's problem at Send, '' when there is none: nothing but spaces, or over 500
 * characters. Counted as the server counts: line breaks as \n, trimmed, and in characters, so an
 * emoji counts once.
 */
export function secondLookTextProblem(text: string): string {
  const count = [...text.replace(/\r\n/g, '\n').trim()].length;
  if (count === 0) return EMPTY_LINE;
  if (count > SECOND_LOOK_MAX_CHARS) return TOO_LONG_LINE;
  return '';
}

/**
 * Where a failed send's line shows, and the line: under the text box, under the photos, or over
 * Send. A null line is the client's own words (no connection, and a photo that would not upload).
 */
export type SecondLookFailure = { at: 'text' | 'photos' | 'form'; line: string | null };

/**
 * A failed send, by the answer's status, error code, and the field and reason of a 400. The server
 * refuses a photo it will not take (one it already holds for something else): the page then
 * uploads the photos again at the next Send. A 409 says the removal was asked about already, is
 * past its 30 days, or was answered by a Bring it back; that last line never says the post is back,
 * since the same answer comes for an older removal of a post that was removed again.
 */
export function secondLookFailure(
  kind: RemovalKind,
  status: number,
  code: string,
  field = '',
  reason = ''
): SecondLookFailure {
  if (code === 'validation_failed' && field === 'text') {
    if (reason === 'required') return { at: 'text', line: EMPTY_LINE };
    if (reason === 'too_long') return { at: 'text', line: TOO_LONG_LINE };
    // A control character, such as a pasted tab: the entry form's line.
    return { at: 'text', line: "That has a character we can't use." };
  }
  if (code === 'validation_failed' && field === 'photo_keys') {
    return { at: 'photos', line: "Couldn't use those photos. Please try again." };
  }
  if (code === 'photo_rejected' || code === 'photo_upload_failed') return { at: 'photos', line: null };
  if (status === 404 || code === 'not_found') {
    return { at: 'form', line: kind === 'post_removed' ? "This post isn't here any more." : "This comment isn't here any more." };
  }
  if (code === 'second_look_asked') return { at: 'form', line: 'You already asked for a second look.' };
  if (code === 'second_look_too_late') return { at: 'form', line: "It's too late to ask. It's been 30 days." };
  if (code === 'second_look_back') return { at: 'form', line: 'We already took another look.' };
  // A 429 without the usual body has no code, so the status counts too.
  if (status === 429 || code === 'rate_limited') {
    return { at: 'form', line: 'Too many tries. Wait a minute and try again.' };
  }
  if (code === 'network_unavailable') return { at: 'form', line: null };
  return { at: 'form', line: 'Something went wrong. Please try again.' };
}

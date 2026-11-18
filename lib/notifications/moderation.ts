/**
 * The bell's items about your own post or comment under moderation: an admin's decision
 * (2026-11-17), or the reports that hid it while we take a look (2026-11-18). A removal names the
 * reason the admin gave (2026-11-18), which the server sends as a report reason's key. And the
 * email a removed one opens to ask for a second look: second looks are by email, with no button for
 * them in the app. The address is the server's setting (CONTACT_EMAIL), sent on the two removed
 * kinds as `contact`; the app never holds one of its own. None of these items has a person.
 * Pure: no imports, so a node script can check it (scripts/check-social.ts).
 */

/** The six kinds, as the server names them. */
export const DECISION_KINDS = [
  'post_removed',
  'comment_removed',
  'post_back',
  'comment_back',
  'post_hidden',
  'comment_hidden',
] as const;

export type DecisionKind = (typeof DECISION_KINDS)[number];

export function isDecisionKind(kind: string): kind is DecisionKind {
  return (DECISION_KINDS as readonly string[]).includes(kind);
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
 * the reports that had hidden it. Hidden: 3 reports hid it until the admin looks. Only a removal
 * reads `reason`.
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
    case 'post_hidden':
      return 'Your post is hidden while we take a look.';
    case 'comment_hidden':
      return 'Your comment is hidden while we take a look.';
  }
}

/** What a second look's email needs: the removed item, and who is asking. */
export type SecondLook = {
  kind: 'post_removed' | 'comment_removed';
  /** The server's contact address, '' when none is set. */
  contact: string;
  /** Your username, without the @. */
  username: string;
  /** The post's title or the comment's words, as the item has them. */
  text: string;
  /** The short ref the admin page shows for the removal. */
  ref: string;
};

/**
 * The email's subject and body lines; the body ends on the line they write under. A comment that
 * was only a photo has no words, so its line says so.
 */
export function secondLookEmail(look: SecondLook): { subject: string; lines: string[] } {
  const post = look.kind === 'post_removed';
  const what = post || look.text.trim() !== '' ? look.text : 'a photo';
  return {
    subject: post ? 'Second look at my post' : 'Second look at my comment',
    lines: [
      `Username: @${look.username}`,
      `${post ? 'Post' : 'Comment'}: ${what}`,
      `Ref: ${look.ref}`,
      '',
      'Why it should come back:',
    ],
  };
}

/** One plain address: anything that would change the link's meaning (?, &, #, a space) is refused. */
const ADDRESS = /^[^\s@?&#%/,;:<>"]+@[^\s@?&#%/,;:<>"]+$/;

/**
 * The mailto: link of a second look, or null when there is no usable address (the item still
 * shows, and a tap on it does nothing). The subject and body are percent-encoded, so a title with
 * ? & # or a line break stays inside the body, and the body's lines end in CRLF, as mailto wants.
 */
export function secondLookMailto(look: SecondLook): string | null {
  const to = look.contact.trim();
  if (!ADDRESS.test(to)) return null;
  const { subject, lines } = secondLookEmail(look);
  return `mailto:${to}?subject=${encodeURIComponent(subject)}&body=${encodeURIComponent(lines.join('\r\n'))}`;
}

/** The line in place of the email when no mail app opens (the iOS Simulator has none). */
export function secondLookFallback(contact: string): string {
  return `Email ${contact.trim()} to ask for a second look.`;
}

/** What a screen reader adds on a removed item that opens the email. */
export const SECOND_LOOK_HINT = 'Opens an email to ask for a second look.';

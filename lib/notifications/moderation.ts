/**
 * The bell's items about an admin's decision on your own post or comment (moderation, 2026-11-17),
 * and the email a removed one opens to ask for a second look: second looks are by email, with no
 * button for them in the app. The address is the server's setting (CONTACT_EMAIL), sent on the two
 * removed kinds as `contact`; the app never holds one of its own. None of these items has a person.
 * Pure: no imports, so a node script can check it (scripts/check-social.ts).
 */

/** The four kinds, as the server names them. */
export const DECISION_KINDS = ['post_removed', 'comment_removed', 'post_back', 'comment_back'] as const;

export type DecisionKind = (typeof DECISION_KINDS)[number];

export function isDecisionKind(kind: string): kind is DecisionKind {
  return (DECISION_KINDS as readonly string[]).includes(kind);
}

/**
 * The row's sentence. Removed: it broke the rules. Back: the admin brought it back, or dismissed
 * the reports that had hidden it.
 */
export function decisionLine(kind: DecisionKind): string {
  const what = kind === 'post_removed' || kind === 'post_back' ? 'post' : 'comment';
  return kind === 'post_removed' || kind === 'comment_removed'
    ? `We removed your ${what}. It broke our rules.`
    : `Sorry, we got that wrong. Your ${what} is back.`;
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

/** The email's subject and body lines; the body ends on the line they write under. */
export function secondLookEmail(look: SecondLook): { subject: string; lines: string[] } {
  const post = look.kind === 'post_removed';
  return {
    subject: post ? 'Second look at my post' : 'Second look at my comment',
    lines: [
      `Username: @${look.username}`,
      `${post ? 'Post' : 'Comment'}: ${look.text}`,
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

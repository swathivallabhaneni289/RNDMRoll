/**
 * The pure rules of the Settings pages (plan 02.1-02). No React and no app imports, so a plain
 * node script can check them (scripts/check-settings.ts of the plan).
 *
 * The numbers repeat rules that live in other places, and all of them must change together:
 *   - the username pattern: the server (internal/user). Sign-up and the Edit page
 *     (components/profile/ProfileForm.tsx) judge a username only through usernameProblem below;
 *   - a password of 8 characters up to 72 BYTES (bcrypt's limit), with a capital letter, a number and a symbol: the server's
 *     auth.PasswordReason (internal/auth) and ProfileForm.tsx;
 *   - a bio of 160 characters: the server's profile check and ProfileForm.tsx.
 * The words are the strings of 02.1-UI-SPEC section 10, verbatim, except the username lines and
 * "Enter your current password." (2026-11-16), which the spec does not have yet.
 */

export const USERNAME_PATTERN = /^[a-z0-9_]{3,20}$/;
export const USERNAME_MIN_LENGTH = 3;
export const USERNAME_MAX_LENGTH = 20;
export const PASSWORD_MIN_LENGTH = 8;
export const PASSWORD_MAX_BYTES = 72;
export const BIO_MAX = 160;

export const LOOKS_GOOD_MESSAGE = 'Looks good.';

/** The username lines name the problem (2026-11-16), in place of the spec's one rule line. */
export const USERNAME_DOT_MESSAGE = 'No dots. Use _ instead.';
export const USERNAME_SPACE_MESSAGE = 'No spaces. Use _ instead.';
export const USERNAME_DASH_MESSAGE = 'No dashes. Use _ instead.';
/** The box already shows an @ in front. */
export const USERNAME_AT_MESSAGE = 'Leave out the @.';
/** Names the set the rule allows, so an é or a Cyrillic letter isn't told it is not a letter. */
export const USERNAME_CHARACTER_MESSAGE = 'Only a to z, 0 to 9 and _.';
export const USERNAME_EMPTY_MESSAGE = 'Pick a username.';
export const USERNAME_SHORT_MESSAGE = 'At least 3 characters.';
/**
 * Said at once, while typing too. The username box has no length cap, because a cap cuts a long
 * paste short without a word and the cut name could then pass the live check.
 */
export const USERNAME_LONG_MESSAGE = 'At most 20 characters.';
export const USERNAME_TAKEN_MESSAGE = "That username's taken. Try one of these:";
export const USERNAME_TAKEN_NO_ALTERNATES_MESSAGE = "That username's taken.";
export const USERNAME_CHECK_FAILED_MESSAGE = "Couldn't check that username. Try again.";
export const USERNAME_NOTICE = 'Username changed.';

export const PASSWORD_SHORT_MESSAGE = 'Password must be at least 8 characters.';
export const PASSWORD_WEAK_MESSAGE = 'Add a capital letter, a number and a symbol.';
export const PASSWORD_LONG_MESSAGE = 'That password is too long. Use 72 characters or fewer.';
export const PASSWORD_SAME_MESSAGE = 'Choose a password that is different from your current one.';
/** Said on a tap of the grey Save password while the Current password box is empty. */
export const CURRENT_PASSWORD_EMPTY_MESSAGE = 'Enter your current password.';
export const CURRENT_PASSWORD_WRONG_MESSAGE = 'That is not your current password.';
export const NEW_PASSWORD_REFUSED_MESSAGE = "That password can't be used. Try a different one.";
export const PASSWORD_NOTICE = 'Password changed. Other phones are signed out.';

export const BIO_LONG_MESSAGE = 'Bio must be 160 characters or fewer.';

/** For a failure that is not an ApiError at all (the client turns every server answer into one). */
export const GENERIC_MESSAGE = 'Something went wrong. Please try again.';

/**
 * The length of the text in UTF-8 bytes. bcrypt refuses more than 72 BYTES and a text field's
 * maxLength counts UTF-16 units, so the byte count is taken by hand.
 */
export function utf8Length(value: string): number {
  let bytes = 0;
  for (const ch of value) {
    const cp = ch.codePointAt(0) ?? 0;
    bytes += cp < 0x80 ? 1 : cp < 0x800 ? 2 : cp < 0x10000 ? 3 : 4;
  }
  return bytes;
}

/** The first thing in the value a username can't have, as its line, or '' when there is none. */
function usernameCharacterProblem(value: string): string {
  if (value.startsWith('@')) return USERNAME_AT_MESSAGE;
  for (const ch of value) {
    if (/[a-z0-9_]/.test(ch)) continue;
    if (ch === '.') return USERNAME_DOT_MESSAGE;
    if (/\s/.test(ch)) return USERNAME_SPACE_MESSAGE;
    // Any dash, so one the keyboard turned into a longer dash is named the same.
    if (/\p{Pd}/u.test(ch)) return USERNAME_DASH_MESSAGE;
    return USERNAME_CHARACTER_MESSAGE;
  }
  return '';
}

/**
 * What to say about a username box, or '' when there is nothing to say. The stored name (the Edit
 * page's) is never judged against itself. Anything else is judged as the box holds it. The box
 * lowers capitals as they are typed, so a capital here counts as a character a username can't have.
 *   - A leading @, or the first character that is not a to z, 0 to 9 or _ (a dot, a space and a
 *     dash are named), and over 20 characters: said at once, even while `typing`, since more
 *     typing can't fix them.
 *   - Empty, or under 3 characters: said only once the person is 'done' (they leave the box, or tap
 *     the grey button), never while the first letters are typed.
 * Once done it is '' exactly when the value is a username (USERNAME_PATTERN) or the stored one, so a
 * grey button always has a line to show. Whether a well formed name is free is the live check's job.
 */
export function usernameProblem(value: string, stored: string, when: 'typing' | 'done' = 'done'): string {
  if (stored !== '' && value === stored) return '';
  const character = usernameCharacterProblem(value);
  if (character !== '') return character;
  // Only a to z, 0 to 9 and _ are left, so the length is the number of characters.
  if (value.length > USERNAME_MAX_LENGTH) return USERNAME_LONG_MESSAGE;
  if (when === 'typing') return '';
  if (value === '') return USERNAME_EMPTY_MESSAGE;
  if (value.length < USERNAME_MIN_LENGTH) return USERNAME_SHORT_MESSAGE;
  return '';
}

export type PasswordCheck = { label: string; met: boolean };

/**
 * The four things a new password needs, in the order the checklist shows them. The server's
 * auth.PasswordReason (internal/auth/password.go) holds the same rule: 8 characters, a capital letter,
 * a number and a symbol (any punctuation or symbol character).
 */
export function passwordChecks(value: string): PasswordCheck[] {
  const chars = Array.from(value);
  return [
    { label: 'At least 8 characters', met: chars.length >= PASSWORD_MIN_LENGTH },
    { label: 'One capital letter', met: /\p{Lu}/u.test(value) },
    { label: 'One number', met: /\p{Nd}/u.test(value) },
    { label: 'One symbol', met: /[\p{P}\p{S}]/u.test(value) },
  ];
}

/**
 * '' for a good new password, else the short message, the long one (over 72 BYTES) or the weak
 * one (a capital letter, a number or a symbol is missing).
 */
export function passwordProblem(value: string): string {
  if (Array.from(value).length < PASSWORD_MIN_LENGTH) return PASSWORD_SHORT_MESSAGE;
  if (utf8Length(value) > PASSWORD_MAX_BYTES) return PASSWORD_LONG_MESSAGE;
  if (passwordChecks(value).some((check) => !check.met)) return PASSWORD_WEAK_MESSAGE;
  return '';
}

/** A new password: the rules above, and it must differ from the current one. */
export function newPasswordProblem(next: string, current: string): string {
  const problem = passwordProblem(next);
  if (problem !== '') return problem;
  return next === current ? PASSWORD_SAME_MESSAGE : '';
}

/** '' while the bio fits (trimmed, as it is sent), else the long message. */
export function bioProblem(value: string): string {
  return Array.from(value.trim()).length > BIO_MAX ? BIO_LONG_MESSAGE : '';
}

/** True when two bios are the same once the ends are trimmed (what Save compares). */
export function sameText(a: string, b: string): boolean {
  return a.trim() === b.trim();
}

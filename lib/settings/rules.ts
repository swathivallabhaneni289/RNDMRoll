/**
 * The pure rules of the Settings pages (plan 02.1-02). No React and no app imports, so a plain
 * node script can check them (scripts/check-settings.ts of the plan).
 *
 * The numbers repeat rules that live in other places, and all of them must change together:
 *   - the username pattern: the server (internal/user) and components/profile/ProfileForm.tsx;
 *   - a password of 8 characters up to 72 BYTES (bcrypt's limit): the server's passwordReason
 *     (internal/httpapi) and ProfileForm.tsx;
 *   - a bio of 160 characters: the server's profile check and ProfileForm.tsx.
 * The words are the strings of 02.1-UI-SPEC section 10, verbatim.
 */

export const USERNAME_PATTERN = /^[a-z0-9_]{3,20}$/;
export const PASSWORD_MIN_LENGTH = 8;
export const PASSWORD_MAX_BYTES = 72;
export const BIO_MAX = 160;

export const LOOKS_GOOD_MESSAGE = 'Looks good.';

export const USERNAME_FORMAT_MESSAGE = '3 to 20 letters, numbers, or underscores.';
export const USERNAME_TAKEN_MESSAGE = "That username's taken. Try one of these:";
export const USERNAME_TAKEN_NO_ALTERNATES_MESSAGE = "That username's taken.";
export const USERNAME_CHECK_FAILED_MESSAGE = "Couldn't check that username. Try again.";
export const USERNAME_NOTICE = 'Username changed.';

export const PASSWORD_SHORT_MESSAGE = 'Password must be at least 8 characters.';
export const PASSWORD_LONG_MESSAGE = 'That password is too long. Use 72 characters or fewer.';
export const PASSWORD_SAME_MESSAGE = 'Choose a password that is different from your current one.';
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

/**
 * What to say about a username box: '' when there is nothing to say (the box is empty, or holds
 * the stored name, which is never "taken" against itself), otherwise the format message when the
 * value does not match the pattern. Whether a well formed name is free is the live check's job.
 */
export function usernameProblem(value: string, stored: string): string {
  if (value === '' || value === stored) return '';
  return USERNAME_PATTERN.test(value) ? '' : USERNAME_FORMAT_MESSAGE;
}

/** '' for a good password, else the short or the long message (long means over 72 BYTES). */
export function passwordProblem(value: string): string {
  if (Array.from(value).length < PASSWORD_MIN_LENGTH) return PASSWORD_SHORT_MESSAGE;
  if (utf8Length(value) > PASSWORD_MAX_BYTES) return PASSWORD_LONG_MESSAGE;
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

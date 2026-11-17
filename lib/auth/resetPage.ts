import { ApiError } from '@/lib/api/client';

/**
 * The Forgot password page's rules about the server's answers (app/(auth)/forgot-password.tsx),
 * kept here, out of the page, so scripts/check-accounts-extra.ts can run them in node.
 */

/**
 * The lines under the Code box, one for each answer about the code (the server's reason). The phone
 * does not count tries (it cannot see the server's count), so each line is true whatever the server
 * holds, and none says whether the email has an account. The server keeps wrong tries across new
 * codes until the code expires (30 minutes after the last code went out), so the line says so.
 */
export const WRONG_CODE =
  "That code isn't right or has expired. Wrong tries add up, even across new codes. After 5, you can send a new code 30 minutes after the last one was sent.";
export const REPLACED = 'That code was replaced by a newer one. Use the newest code in your email.';
export const USED = 'That code was already used. If your password changed, log in with the new one.';
export const SHORT_CODE = 'Enter the 6-digit code from the email.';

/** The line for the server's reason about the code; an unknown one reads as a wrong code. */
export function codeLine(reason: string | undefined): string {
  if (reason === 'replaced') return REPLACED;
  if (reason === 'used') return USED;
  if (reason === 'invalid') return SHORT_CODE;
  return WRONG_CODE;
}

/**
 * True when a field "code" answer, of the check or of a reset, says MAYBE_CHANGED_AFTER instead of
 * its line: after a reset whose answer was lost, every reason but replaced and invalid. One rule for
 * both requests, so they never disagree.
 */
export function maybeSpentByLostReset(reason: string | undefined, afterLostAnswer: boolean): boolean {
  return afterLostAnswer && reason !== 'replaced' && reason !== 'invalid';
}

/** Too many requests: by the HTTP status too, as a 429 without the usual body has no code. */
export function tooMany(err: unknown): boolean {
  return err instanceof ApiError && (err.status === 429 || err.code === 'rate_limited');
}

/** No answer came (no connection, the client's own time limit) or the server failed. */
export function answerLost(err: unknown): boolean {
  return !(err instanceof ApiError) || err.status === 0 || err.status >= 500;
}

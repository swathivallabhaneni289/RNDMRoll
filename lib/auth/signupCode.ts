/**
 * The sign-up code page's lines (app/(auth)/signup-code.tsx), and the rules that pick them, kept
 * here, out of the page, so scripts/check-auth-today.ts can check them in node. No React and no app
 * imports. The phone does not count tries (it cannot see the server's count), so each line is true
 * whatever the server holds.
 */

/** The line under the heading. It names the address, so a typo shows. */
export function sentLine(email: string, resent: boolean): string {
  return resent ? `We sent a new code to ${email}.` : `We sent a code to ${email}.`;
}

// The lines under the Code box, one for each answer about the code (the server's reason). Wrong
// tries add up across new codes, so after 5 only a new sign-up (Continue on page 1) gets new ones.
export const SHORT_CODE = 'Enter the 6-digit code from the email.';
export const WRONG_CODE = "That code isn't right. After 5 wrong tries, go back and tap Continue for a new code.";
export const REPLACED = 'That code was replaced by a newer one. Use the newest code in your email.';
/** The sign-up is gone: its 30 minutes are up. */
export const EXPIRED = 'That code has expired. Go back and tap Continue for a new one.';

// The lines under the boxes, for answers that are not about the code.
/** Send a new code once the sign-up is over: it expired, its tries are used up, or 5 codes went out. */
export const NO_MORE_CODES = 'No more codes for this sign-up. Go back and tap Continue.';
/** Verify: an account got the email since Continue on page 1. */
export const EMAIL_TAKEN = 'That email is already in use.';
/** Verify: someone took the username since page 1 checked it. */
export const USERNAME_TAKEN = 'Someone just took that username. Go back and pick another.';
/** A 429: the server's limits count requests here, not wrong codes. */
export const TOO_MANY = 'Too many requests. Wait a minute and try again.';
export const GENERIC = 'Something went wrong. Please try again.';

/** What these rules read of a failed request (an ApiError's status, code, field and reason). */
export interface CodeFailure {
  status: number;
  code: string;
  field?: string;
  reason?: string;
}

/** The line for the server's reason about the code; an unknown one reads as a wrong code. */
export function codeLine(reason: string | undefined): string {
  if (reason === 'invalid') return SHORT_CODE;
  if (reason === 'replaced') return REPLACED;
  if (reason === 'expired') return EXPIRED;
  return WRONG_CODE;
}

/** The line for an answer of Verify code or Send a new code that is not about the code. */
export function answerLine(failure: CodeFailure): string {
  if (failure.field === 'signup_token' && failure.reason === 'expired') return NO_MORE_CODES;
  if (failure.code === 'email_taken') return EMAIL_TAKEN;
  if (failure.code === 'username_taken') return USERNAME_TAKEN;
  // By the HTTP status too, as a 429 without the usual body has no code.
  if (failure.status === 429 || failure.code === 'rate_limited') return TOO_MANY;
  return GENERIC;
}

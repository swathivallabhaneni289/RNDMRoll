import type { SignupTry } from '@/lib/auth/signupRetry';

/**
 * The sign-up in progress, handed from page 1 to the code page (developer, 2026-11-24: the account
 * is made only once the emailed code is typed in). In memory only, never in route params or storage,
 * as it holds the password: an app restart forgets it, and the code page then sends the person back
 * to page 1 to start again. No React and only a type import, so a plain node script can check it
 * (scripts/check-auth-today.ts).
 */

/** A started sign-up: what page 1 sent, and the token the server gave it. */
export interface PendingSignup extends SignupTry {
  /** From POST /auth/signup: names this sign-up at verify and at Send a new code. */
  token: string;
}

let pending: PendingSignup | null = null;
// The last sign-up whose verify may have made an account this phone is not signed in to: its
// answer was lost, or the session it brought could not be kept (lib/auth/signupRetry.ts). A new
// sign-up from page 1 keeps it, as the server can finish that verify's insert after the new start
// has passed its email check. Cleared once signed in.
let lost: SignupTry | null = null;

/** The sign-up in progress, or null: none started since the app did, or it signed in. */
export function pendingSignup(): PendingSignup | null {
  return pending;
}

/** Continue on page 1 started a sign-up: it takes the place of any earlier one. */
export function setPendingSignup(attempt: SignupTry, token: string): void {
  pending = { ...attempt, token };
}

/** The sign-up whose verify may have made the account, or null. */
export function lostSignup(): SignupTry | null {
  return lost;
}

/** A verify of this sign-up may have made the account. */
export function setLostSignup(attempt: SignupTry): void {
  lost = attempt;
}

/** Signed in: the sign-up is over, and its password leaves memory. */
export function clearPendingSignup(): void {
  pending = null;
  lost = null;
}

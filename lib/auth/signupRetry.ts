/**
 * Sign-up's way back to an account its own lost answer made. No React and no app imports, so a plain
 * node script can check it (scripts/check-auth-today.ts).
 *
 * Continue on page 1 only starts a sign-up, and the code page's verify makes the account once the
 * emailed code is right (developer, 2026-11-24). A start makes no account, so a start whose answer
 * was lost needs nothing: the next Continue starts again. A verify that heard nothing back, or
 * heard a server failure, may still have made the account, as the server can finish the insert
 * after the phone stops waiting and the session it then makes can fail on its own. So may a verify
 * whose session this phone could not keep. That sign-up is then finished: a later verify of it is
 * answered "expired", and a later Continue with the same email is answered email_taken, both by the
 * person's own new account. The page then logs in with that sign-up's email and password instead of
 * showing its line. lib/auth/pendingSignup.ts keeps the lost sign-up until someone signs in.
 */

/** A sign-up, as it was sent. */
export interface SignupTry {
  email: string;
  password: string;
  username: string;
}

/** What these rules read of a failed request (an ApiError's status, code and reason). */
export interface SignupFailure {
  status: number;
  code: string;
  /** The server's word for what is wrong with the field it names, on a 400. */
  reason?: string;
}

/**
 * True when a verify's answer was lost: none came back (status 0: no connection, or the phone's own
 * 10 second limit) or the server failed (5xx), which can happen after the insert.
 */
export function signupAnswerLost(failure: SignupFailure): boolean {
  return failure.status === 0 || failure.status >= 500;
}

/** Emails match trimmed and ignoring capitals, as the server's unique check compares them. */
function sameEmail(a: string, b: string): boolean {
  return a.trim().toLowerCase() === b.trim().toLowerCase();
}

/**
 * True when an answer may be the person's own new account: a verify whose answer was lost (`lost`)
 * may have made it, this try has the same email, ignoring capitals, and the same password, and the
 * answer is email_taken (an account has that email now) or "expired" (the sign-up is gone, as that
 * verify may have finished it). One rule for Continue on page 1 and for the code page's verify, so
 * the two never disagree.
 */
export function shouldTryLogin(
  lost: SignupTry | null,
  email: string,
  password: string,
  failure: SignupFailure
): boolean {
  return (
    lost !== null &&
    (failure.code === 'email_taken' || failure.reason === 'expired') &&
    sameEmail(lost.email, email) &&
    lost.password === password
  );
}

/**
 * After that log in, the photo-and-bio page follows as after a sign-up only when the account is the
 * one the lost sign-up asked for (its username). An older account with that email and password, or
 * one the log in brought back from deletion, signs in like Log in does.
 */
export function opensExtras(
  lost: SignupTry,
  result: { restored?: boolean; user: { username: string | null } }
): boolean {
  return result.restored !== true && result.user.username === lost.username;
}

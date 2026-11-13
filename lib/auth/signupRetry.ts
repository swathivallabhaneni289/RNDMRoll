/**
 * Sign-up's way back to an account its own lost answer made. No React and no app imports, so a plain
 * node script can check it (scripts/check-auth-today.ts).
 *
 * The server finishes the account insert even after the phone stops waiting (Signup in
 * internal/httpapi/auth.go), and the session it then makes can fail on its own. So a try that heard
 * nothing back, or heard a server failure, may still have made the account, and the next Continue
 * with the same email is answered email_taken by the person's own new account. The form then logs
 * in with that try's email and password instead of saying "That email is already in use.".
 */

/** A sign-up try, as it was sent. */
export interface SignupTry {
  email: string;
  password: string;
  username: string;
}

/** What these rules read of a failed request (an ApiError's status and code). */
export interface SignupFailure {
  status: number;
  code: string;
}

/**
 * True when a try's answer was lost: none came back (status 0: no connection, or the phone's own
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
 * True when an email_taken answer may be the person's own new account: an earlier try lost its
 * answer (`lost`), and this try has the same email, ignoring capitals, and the same password.
 */
export function shouldTryLogin(
  lost: SignupTry | null,
  email: string,
  password: string,
  failure: SignupFailure
): boolean {
  return lost !== null && failure.code === 'email_taken' && sameEmail(lost.email, email) && lost.password === password;
}

/**
 * After that log in, the photo-and-bio page follows as after a sign-up only when the account is the
 * one the lost try asked for (its username). An older account with that email and password, or one
 * the log in brought back from deletion, signs in like Log in does.
 */
export function opensExtras(
  lost: SignupTry,
  result: { restored?: boolean; user: { username: string | null } }
): boolean {
  return result.restored !== true && result.user.username === lost.username;
}

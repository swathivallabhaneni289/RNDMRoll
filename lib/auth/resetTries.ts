/**
 * Forgot password codes asked for and typed wrong, kept for each email while the app runs. The
 * server's rules (internal/httpapi/password_reset.go): every code typed counts as a try, a new
 * code keeps the tries of the one it replaces, and after 5 the code is used up: no new one is sent
 * until it expires, 30 minutes after it went out. Only a code sent after that starts again from 0
 * tries. The server answers the same whatever happens, so this record is how the Forgot password
 * page knows not to point at a used-up code. It lives in memory only: leaving and reopening the
 * page keeps it, a relaunch forgets it.
 *
 * Wrong codes are counted the same whether or not the email has an account (the server answers
 * both alike), so nothing shown from this record says which emails have one.
 * No React and no app imports, so a plain node script can check it.
 */

/** The server's tries for one code, and how long a code lives. */
export const CODE_TRIES = 5;
export const CODE_LIFE_MS = 30 * 60 * 1000;

/**
 * askedAt: when a code was last asked for on this phone. The code in use went out then or a little
 * before (no new one comes within a minute of the last), so it expires by 30 minutes after it.
 */
type CodeTries = { wrong: number; askedAt: number };

/** By email, trimmed and lowercased as the server reads it. */
const byEmail = new Map<string, CodeTries>();

function keyOf(email: string): string {
  return email.trim().toLowerCase();
}

function usedUp(entry: CodeTries | undefined, now: number): boolean {
  return entry !== undefined && entry.wrong >= CODE_TRIES && now - entry.askedAt < CODE_LIFE_MS;
}

/**
 * The server answered a request for a new code for email. 30 minutes or more after the last one
 * the code in use has expired, so the new one starts from 0 tries; sooner, it keeps the tries.
 */
export function noteCodeAsked(email: string, now: number = Date.now()): void {
  const key = keyOf(email);
  const entry = byEmail.get(key);
  if (entry === undefined || now - entry.askedAt >= CODE_LIFE_MS) byEmail.set(key, { wrong: 0, askedAt: now });
  else entry.askedAt = now;
}

/**
 * Counts one wrong code for email (the server's answer named the code). True when the code is now
 * used up. With no code asked for here, the first wrong code stands in for when it went out.
 */
export function noteWrongCode(email: string, now: number = Date.now()): boolean {
  const key = keyOf(email);
  const entry = byEmail.get(key) ?? { wrong: 0, askedAt: now };
  entry.wrong += 1;
  byEmail.set(key, entry);
  return usedUp(entry, now);
}

/**
 * True from the fifth wrong code until 30 minutes after the code was last asked for: then the code
 * has expired and a new one can be sent. A fifth wrong code typed after that never counted on the
 * server, so it is not used up either.
 */
export function codeUsedUp(email: string, now: number = Date.now()): boolean {
  return usedUp(byEmail.get(keyOf(email)), now);
}

/** A reset that worked: the code is spent, and the next one starts from 0 tries. */
export function noteCodeSpent(email: string): void {
  byEmail.delete(keyOf(email));
}

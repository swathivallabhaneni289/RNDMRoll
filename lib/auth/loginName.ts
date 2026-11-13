/**
 * What Log in sends for the "Email or username" box. No React and no app imports, so a plain node
 * script can check it (scripts/check-auth-today.ts).
 *
 * The app writes every username as "@name" (Profile, a person's page, People), so a person may
 * type theirs that way. One leading @ before a word of letters, digits and _ is dropped, as Search
 * and the server's lookups (social.CleanUsername) already do, and the rest is looked up as a
 * username; the server's Log in drops it the same way (LoginRequest.typed). Any other @ keeps the
 * text an email address: "@" alone, "a@b.co", "@a@b.co" and "@nope.com" are sent as typed and get
 * the address check.
 */

/** One @ at the start, then one word of letters, digits and _ (what a username is made of). */
const AT_USERNAME = /^@[A-Za-z0-9_]+$/;

/** The typed name as it is checked and sent: trimmed, without a username's one leading @. */
export function loginName(typed: string): string {
  const value = typed.trim();
  return AT_USERNAME.test(value) ? value.slice(1) : value;
}

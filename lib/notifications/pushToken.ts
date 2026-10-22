import { Platform } from 'react-native';
import Constants from 'expo-constants';
import { api, isRetryable } from '@/lib/api/client';
import { loadNotifications } from '@/lib/notifications/module';

/**
 * The phone's Expo push token, kept on the server so it can send pushes while the app is closed
 * (docs/PUSH-SETUP.md). The server keeps it for the session whose access token sent it last.
 * Everything here is quiet and best effort:
 *  - Nothing happens without an Expo project id in app.json (extra.eas.projectId, written by
 *    `eas init`), without the notification module in this build, or without the phone's permission
 *    (the reminder's one-time ask at the start covers it).
 *  - It never shows anything to the person and never throws.
 * This file does not import the session store (the store imports it for sign in and log out).
 */

/** The Expo project id from app.json, or null before `eas init`. */
export function expoProjectId(): string | null {
  const extra = Constants.expoConfig?.extra as { eas?: { projectId?: unknown } } | undefined;
  const fromExtra = extra?.eas?.projectId;
  if (typeof fromExtra === 'string' && fromExtra !== '') return fromExtra;
  const fromEas = (Constants as unknown as { easConfig?: { projectId?: unknown } }).easConfig?.projectId;
  return typeof fromEas === 'string' && fromEas !== '' ? fromEas : null;
}

/**
 * A token already sent is sent again after this long, so a server that forgot it while this person
 * stayed signed in (an admin's disable forgets every token of the account) gets it back.
 */
const RESEND_AFTER_MS = 12 * 60 * 60 * 1000;

/**
 * How long a registration gives the phone to hand over its token (it asks Expo's server for it,
 * which can be slow on a poor connection). A token that comes within it is still sent, as long as
 * nothing changed meanwhile (no log out, no sign in). After that the run ends, and the next start
 * or return to the front tries again.
 */
const REGISTER_TOKEN_LIMIT_MS = 60_000;

/**
 * How long a log out waits for the phone's token when this run has sent no POST (after a relaunch,
 * the server may still have the token from the last run). After that it goes on without, so a log
 * out never hangs on the phone.
 */
const TOKEN_LIMIT_MS = 3000;

/** One POST of the token: for whom, whether it went through, and when it did (null until then). */
type Registration = { userId: string; token: string; sent: Promise<boolean>; sentAt: number | null };

/**
 * The last token POSTed for the person signed in. It is kept as soon as its POST starts, so a log
 * out meanwhile can wait for it, and kept when that POST fails: a POST that seemed to fail (the
 * client's own 10 second limit, an answer lost on the way) may still have been stored, so log out
 * asks the server to forget the token either way. Only a POST that went through less than 12
 * hours ago stops a repeat.
 */
let registered: Registration | null = null;
/** The registration running now and for whom: a second call for the same person shares it. */
let inFlight: { userId: string; done: Promise<void> } | null = null;
/**
 * Bumped by every sign in (beginSignIn): a registration begun before one sends nothing, and a log
 * out begun before one sends no DELETE.
 */
let generation = 0;
/** From a log out until the next sign in: nothing more is sent for the account that is leaving. */
let loggedOut = false;
/** The person signed in now: named by beginSignIn, or after a relaunch by the first registration. */
let signedInUserId: string | null = null;
/**
 * Calls of the token sent and not answered yet: each POST and whom it was for, and each DELETE. The
 * server keeps the token for the session whose POST it got last, so a POST goes out only once every
 * call still on its way has been answered (an older session's POST, or a DELETE, must not land
 * after the newest POST), and a log out's DELETE waits for all the POSTs sent for the person
 * leaving (from before a password change too).
 */
const unanswered = new Set<{ userId: string | null; post: boolean; answered: Promise<void> }>();

/** Keeps call in unanswered until it is answered, whatever the answer. */
function track(call: Promise<unknown>, userId: string | null, post: boolean): void {
  const entry = { userId, post, answered: Promise.resolve() };
  const done = () => {
    unanswered.delete(entry);
  };
  entry.answered = call.then(done, done);
  unanswered.add(entry);
}

/** This phone's push token, or null without the module, a project id or the person's permission. */
async function phoneToken(): Promise<string | null> {
  const projectId = expoProjectId();
  const mod = loadNotifications();
  if (projectId === null || mod === null) return null;
  if (Platform.OS !== 'ios' && Platform.OS !== 'android') return null;
  const permission = await mod.getPermissionsAsync();
  if (!permission.granted) return null;
  const { data: token } = await mod.getExpoPushTokenAsync({ projectId });
  return typeof token === 'string' && token !== '' ? token : null;
}

/** phoneToken, or null when it fails or takes longer than ms. Never throws. */
function phoneTokenWithin(ms: number): Promise<string | null> {
  return new Promise((resolve) => {
    const timer = setTimeout(() => resolve(null), ms);
    phoneToken().then(
      (token) => {
        clearTimeout(timer);
        resolve(token);
      },
      () => {
        clearTimeout(timer);
        resolve(null);
      }
    );
  });
}

/**
 * Sends this phone's push token to the server for userId when it can. Safe to call often (every
 * signed-in start and every return to the front): one at a time for each person, only after every
 * call of the token still on its way has been answered, and a token whose POST went through for the
 * same person is not sent again for 12 hours. The phone gets 60 seconds to give its token: one that
 * comes late is still sent if the session is the same, and a run that gets none ends, so the next
 * start or return to the front can try again.
 */
export function registerPushToken(userId: string): Promise<void> {
  if (inFlight !== null && inFlight.userId === userId) return inFlight.done;
  if (signedInUserId === null) signedInUserId = userId;
  const started = generation;
  const done = (async () => {
    try {
      const token = await phoneTokenWithin(REGISTER_TOKEN_LIMIT_MS);
      if (token === null) return;
      // After every call still on its way (a POST, whoever it was for, or a DELETE), so this one
      // lands last.
      while (unanswered.size > 0) await Promise.all(Array.from(unanswered, (p) => p.answered));
      // Logged out, or signed in again, while this ran: it belongs to a session that is over.
      if (loggedOut || generation !== started) return;
      if (
        registered !== null &&
        registered.userId === userId &&
        registered.token === token &&
        registered.sentAt !== null &&
        Date.now() - registered.sentAt < RESEND_AFTER_MS
      ) {
        return;
      }
      // Carries the access token of the session signed in now, which is this one.
      const sent = api.post('/me/push-tokens', { token, platform: Platform.OS }).then(
        () => true,
        () => false
      );
      track(sent, userId, true);
      const record: Registration = { userId, token, sent, sentAt: null };
      registered = record;
      // Counted as sent only when it went through and is still the session's latest: after a
      // failure the next start or return to the front tries again, and a sign in meanwhile wants
      // the token sent again anyway.
      if ((await sent) && generation === started && registered === record) {
        record.sentAt = Date.now();
      }
    } catch {
      // No token this time (a simulator, no network, no credentials yet); the next start tries again.
    }
  })();
  const run = { userId, done };
  inFlight = run;
  // Cleared when it ends, unless a newer one took its place. Not in a finally inside: a call that
  // ends before its first await would run that before the line above.
  void done.then(() => {
    if (inFlight === run) inFlight = null;
  });
  return done;
}

/**
 * A sign in on this phone begins (log in, sign-up, a password change or reset). The store calls it
 * before anything is awaited, together with its own count of sign ins, so the two counts never
 * disagree: anything still running from before sends nothing, and a log out begun before it sends
 * no DELETE. The same person's last token stays known, no longer counted as sent, so a log out
 * still deletes it even if the new session never gets as far as its own POST. The store then sends
 * the token again (registerPushToken), once the new session is in place, even for the same person
 * and token: a password change or reset makes the server forget every token of the account, this
 * phone's too. Gives back an undo for a sign in that fails (a keychain write): everything is as it
 * was before it began, except that what was still running from before stays stopped (the count
 * moves on again rather than back, so nothing begun meanwhile can pass for the session from
 * before).
 */
export function beginSignIn(userId: string): () => void {
  const before = {
    loggedOut,
    signedInUserId,
    registered,
    sentAt: registered === null ? null : registered.sentAt,
  };
  generation += 1;
  const mine = generation;
  loggedOut = false;
  signedInUserId = userId;
  if (registered !== null && registered.userId === userId) registered.sentAt = null;
  else registered = null;
  inFlight = null;
  return () => {
    if (generation !== mine) return;
    generation += 1;
    loggedOut = before.loggedOut;
    signedInUserId = before.signedInUserId;
    registered = before.registered;
    if (registered !== null) registered.sentAt = before.sentAt;
    inFlight = null;
  };
}

/**
 * A log out did not go through (the keychain could not be cleared) and the person stays signed in:
 * the token is sent again from the next start or return to the front.
 */
export function logOutFailed(): void {
  loggedOut = false;
}

/** One DELETE of token. Gives back token when it could not reach the server (isRetryable). */
async function sendDelete(
  token: string,
  opts: { retry?: boolean; bearer?: string },
  userId: string | null
): Promise<string | null> {
  const call = api.delete('/me/push-tokens', opts, { token });
  track(call, userId, false);
  try {
    await call;
    return null;
  } catch (err) {
    return isRetryable(err) ? token : null;
  }
}

/**
 * The DELETE of a log out left over from before (it could not reach the server then): sent with
 * that session's own access token, once, never refreshed (the store sends it only while nobody is
 * signed in). A registration begun meanwhile waits for it. Never throws.
 */
export async function deleteLeftPushToken(token: string, accessToken: string): Promise<void> {
  await sendDelete(token, { bearer: accessToken }, null);
}

/**
 * Log out: asks the server to forget this phone's token, so this phone gets no more pushes for
 * the account that is leaving. Never throws, so log out always goes on.
 *
 * It is awaited to the end (each call is cut off by the client's own 10 second limit, and an
 * offline phone fails at once) rather than raced against a shorter timer, so an expired access
 * token's refresh on the way has finished before the log out goes on (the store keeps the tokens
 * of a refresh that finishes during a log out out of the keychain and memory). Every POST of the
 * token for the person leaving that is still on its way is waited for first, so the DELETE lands
 * after all of them. The DELETE goes out
 * whatever they answered: one that seemed to fail may still have been stored, and deleting a token
 * the server does not have does no harm. When this run has sent no POST (after a relaunch, say),
 * the server may still have the token from before, so the phone is asked for it, for up to 3
 * seconds. The one exception is a sign in while it waited (a failed refresh signed this account out
 * meanwhile, and someone signed in): the token now belongs to that new session, whose sign in sends
 * it again, and a DELETE now would carry the new session's access token and take its token off the
 * server. Gives back the token when its DELETE could not reach the server (no connection, a server
 * error, too many requests), so the store can send it later; null otherwise.
 *
 * mayRefresh is false when a failed refresh is what signs out: that refresh is still pending, so
 * the call waits on neither it nor a POST (which then almost always waits on that same refresh,
 * and fails with it), and the DELETE goes out at once without a refresh. Then nothing is looked up
 * when no POST was sent either: the access token no longer works (that is why it refreshed), so
 * such a DELETE could not go through, and the server sends no pushes for a session that is over.
 */
export async function unregisterPushToken(mayRefresh: boolean): Promise<string | null> {
  const started = generation;
  // Nothing more goes out for the account that is leaving, whatever is still running.
  loggedOut = true;
  const current = registered;
  // Cleared first: a sign-out that this call itself sets off can never send it twice.
  registered = null;
  if (!mayRefresh) {
    if (current === null) return null;
    // retry: true (no refresh) while a refresh is the thing signing out: waiting on it would never
    // end. When it fails it is left on the server, which sends nothing for a session that is over.
    return sendDelete(current.token, { retry: true }, current.userId);
  }
  // Asked for at once, while the POSTs still on their way are answered.
  const lookup = current === null ? phoneTokenWithin(TOKEN_LIMIT_MS) : null;
  // After every POST still on its way for the person leaving, also from before a password change:
  // for the order only, never to skip the DELETE, unless someone signed in meanwhile. Another
  // person's POST does not matter here: this DELETE only takes this person's token off.
  const leaving = current !== null ? current.userId : signedInUserId;
  await Promise.all(
    Array.from(unanswered)
      .filter((p) => p.post && p.userId === leaving)
      .map((p) => p.answered)
  );
  const token = current !== null ? current.token : await lookup;
  if (token === null || generation !== started) return null;
  // When it fails it is left on the server: it moves to the next account that signs in on this
  // phone, and the server sends nothing once the session is over.
  return sendDelete(token, {}, leaving);
}

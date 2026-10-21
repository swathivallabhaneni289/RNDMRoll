import { Platform } from 'react-native';
import Constants from 'expo-constants';
import { api } from '@/lib/api/client';
import { loadNotifications } from '@/lib/notifications/module';

/**
 * The phone's Expo push token, kept on the server so it can send pushes while the app is closed
 * (docs/PUSH-SETUP.md). Everything here is quiet and best effort:
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
 * The token sent to the server and for whom, so a repeat is not sent again. It is kept as soon as
 * its POST starts, with whether that POST went through, so a log out meanwhile can wait for it.
 */
let registered: { userId: string; token: string; sent: Promise<boolean> } | null = null;
/** The registration running now and for whom: a second call for the same person shares it. */
let inFlight: { userId: string; done: Promise<void> } | null = null;
/** Bumped by every sign in: a registration begun before one sends nothing. */
let generation = 0;
/** From a log out until the next sign in: nothing more is sent for the account that is leaving. */
let loggedOut = false;

/**
 * Sends this phone's push token to the server for userId when it can. Safe to call often (every
 * signed-in start and every return to the front): one at a time for each person, and a token
 * already sent for the same person is not sent again.
 */
export function registerPushToken(userId: string): Promise<void> {
  if (inFlight !== null && inFlight.userId === userId) return inFlight.done;
  const started = generation;
  const done = (async () => {
    try {
      const projectId = expoProjectId();
      const mod = loadNotifications();
      if (projectId === null || mod === null) return;
      if (Platform.OS !== 'ios' && Platform.OS !== 'android') return;
      const permission = await mod.getPermissionsAsync();
      if (!permission.granted) return;
      const { data: token } = await mod.getExpoPushTokenAsync({ projectId });
      if (typeof token !== 'string' || token === '') return;
      // Logged out, or signed in again, while this ran: it belongs to a session that is over.
      if (loggedOut || generation !== started) return;
      if (registered !== null && registered.userId === userId && registered.token === token) return;
      const sent = api.post('/me/push-tokens', { token, platform: Platform.OS }).then(
        () => true,
        () => false
      );
      const record = { userId, token, sent };
      registered = record;
      // Forgotten when it did not go through, so the next start tries again.
      if (!(await sent) && registered === record) registered = null;
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
 * A sign in on this phone (log in, sign-up, a password change or reset): sends the token again
 * now, even for the same person and token, because a password change or reset makes the server
 * forget every token of the account, this phone's too. Anything still running from before sends
 * nothing.
 */
export function resendPushToken(userId: string): void {
  generation += 1;
  loggedOut = false;
  registered = null;
  inFlight = null;
  void registerPushToken(userId);
}

/**
 * Log out: asks the server to forget this phone's token, so this phone gets no more pushes for
 * the account that is leaving. Never throws, so log out always goes on. Does nothing when no token
 * was sent.
 *
 * It is awaited to the end (each call is cut off by the client's own 10 second limit, and an
 * offline phone fails at once) rather than raced against a shorter timer: an expired access token
 * is refreshed on the way, and a refresh still running after log out cleared the keychain could
 * write the leaving account's tokens back. A POST of the token still on its way is waited for
 * first, so the DELETE lands after it. mayRefresh is false when a failed refresh is what signs
 * out: that refresh is still pending, so the call waits on neither it nor such a POST (which then
 * almost always waits on that same refresh, and fails with it).
 */
export async function unregisterPushToken(mayRefresh: boolean): Promise<void> {
  // Nothing more goes out for the account that is leaving, whatever is still running.
  loggedOut = true;
  const current = registered;
  // Cleared first: a sign-out that this call itself sets off can never send it twice.
  registered = null;
  if (current === null) return;
  // After its POST, which may still be on its way; nothing to delete when that POST failed.
  if (mayRefresh && !(await current.sent)) return;
  try {
    // retry: true (no refresh) while a refresh is the thing signing out: waiting on it would never end.
    await api.delete('/me/push-tokens', mayRefresh ? {} : { retry: true }, { token: current.token });
  } catch {
    // Left on the server; it moves to the next account that signs in on this phone.
  }
}

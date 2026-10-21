import { Platform } from 'react-native';
import Constants from 'expo-constants';
import type * as NotificationsModule from 'expo-notifications';
import { api } from '@/lib/api/client';

/**
 * The phone's Expo push token, kept on the server so it can send pushes while the app is closed
 * (docs/PUSH-SETUP.md). Everything here is quiet and best effort:
 *  - Nothing happens without an Expo project id in app.json (extra.eas.projectId, written by
 *    `eas init`), without the notification module in this build, or without the phone's permission
 *    (the reminder's one-time ask at the start covers it).
 *  - It never shows anything to the person and never throws.
 * This file does not import the session store (the store imports it for log out).
 */

let notifications: typeof NotificationsModule | null | undefined;

function loadNotifications(): typeof NotificationsModule | null {
  if (notifications !== undefined) return notifications;
  try {
    // eslint-disable-next-line @typescript-eslint/no-require-imports
    notifications = require('expo-notifications') as typeof NotificationsModule;
  } catch {
    notifications = null;
  }
  return notifications;
}

/** The Expo project id from app.json, or null before `eas init`. */
export function expoProjectId(): string | null {
  const extra = Constants.expoConfig?.extra as { eas?: { projectId?: unknown } } | undefined;
  const fromExtra = extra?.eas?.projectId;
  if (typeof fromExtra === 'string' && fromExtra !== '') return fromExtra;
  const fromEas = (Constants as unknown as { easConfig?: { projectId?: unknown } }).easConfig?.projectId;
  return typeof fromEas === 'string' && fromEas !== '' ? fromEas : null;
}

/** The token last sent to the server and for whom, so a repeat is not sent again. */
let registered: { userId: string; token: string } | null = null;
let inFlight: Promise<void> | null = null;

/**
 * Sends this phone's push token to the server for userId when it can. Safe to call often (every
 * signed-in start and every return to the front): one at a time, and a token already sent for the
 * same person is not sent again.
 */
export function registerPushToken(userId: string): Promise<void> {
  if (inFlight !== null) return inFlight;
  inFlight = (async () => {
    try {
      const projectId = expoProjectId();
      const mod = loadNotifications();
      if (projectId === null || mod === null) return;
      if (Platform.OS !== 'ios' && Platform.OS !== 'android') return;
      const permission = await mod.getPermissionsAsync();
      if (!permission.granted) return;
      const { data: token } = await mod.getExpoPushTokenAsync({ projectId });
      if (typeof token !== 'string' || token === '') return;
      if (registered !== null && registered.userId === userId && registered.token === token) return;
      await api.post('/me/push-tokens', { token, platform: Platform.OS });
      registered = { userId, token };
    } catch {
      // No token this time (a simulator, no network, no credentials yet); the next start tries again.
    } finally {
      inFlight = null;
    }
  })();
  return inFlight;
}

/**
 * Log out: asks the server to forget this phone's token, so this phone gets no more pushes for
 * the account that is leaving. Never throws, so log out always goes on. Does nothing when no token
 * was sent.
 *
 * It is awaited to the end (each call is cut off by the client's own 10 second limit, and an
 * offline phone fails at once) rather than raced against a shorter timer: an expired access token
 * is refreshed on the way, and a refresh still running after log out cleared the keychain could
 * write the leaving account's tokens back. mayRefresh is false when a failed refresh is what
 * signs out: that refresh is still pending, so the call must not wait on it.
 */
export async function unregisterPushToken(mayRefresh: boolean): Promise<void> {
  const current = registered;
  // Cleared first: a sign-out that this call itself sets off can never send it twice.
  registered = null;
  if (current === null) return;
  try {
    // retry: true (no refresh) while a refresh is the thing signing out: waiting on it would never end.
    await api.delete('/me/push-tokens', mayRefresh ? {} : { retry: true }, { token: current.token });
  } catch {
    // Left on the server; it moves to the next account that signs in on this phone.
  }
}

import { useEffect } from 'react';
import { AppState } from 'react-native';
import { useRouter } from 'expo-router';
import * as SecureStore from 'expo-secure-store';
import type * as NotificationsModule from 'expo-notifications';

/**
 * The 8 PM reminder (Phase 6, 06-CONTEXT.md "Reminder" and 06-FOLLOW-OVERRIDES.md section 7): a
 * LOCAL phone notification at 8:00 PM in the phone's own time, every day, saying the spin is open.
 * The server is not involved, so no Apple push key is needed.
 *
 * Where things live:
 *  - The switch value and the "asked once" mark are kept on the phone with expo-secure-store, as
 *    the paper says (not in lib/session/store.ts, which keeps only the session). They are plain
 *    flags, not credentials.
 *  - On every signed-in app start `useReminderSetup` (mounted by the signed-in layout) rebuilds the
 *    schedule: cancel everything, then schedule one if the switch is on and the phone allows it.
 *    Every rebuild runs one after another and the one notification has a fixed id, so the reminder
 *    can never be scheduled twice.
 *  - The native module arrives with a dev-client rebuild. Until then (or on any build without it)
 *    the module is loaded lazily and every call is guarded: the reminder is simply unavailable, the
 *    switch shows off, there is no ask, and the app starts as before.
 */

/** 8:00 PM, in the phone's own time zone. */
export const REMINDER_HOUR = 20;
export const REMINDER_MINUTE = 0;

export const REMINDER_TITLE = 'RNDMRoll';
export const REMINDER_BODY = 'Your spin is open.';

/** Tapping the reminder opens the Spin tab. */
export const REMINDER_ROUTE = '/(app)/today';

/** The one scheduled notification's id: scheduling again replaces it instead of adding one. */
export const REMINDER_ID = 'rndmroll-daily-spin-reminder';

/** The switch's label on Settings > Notifications. */
export const REMINDER_SWITCH_LABEL = 'Remind me at 8:00 PM';

/** The one line under the switch while the phone's own permission is off. */
export const PERMISSION_OFF_MESSAGE = 'Turn on notifications for RNDMRoll in Settings.';

const REMINDER_ON_KEY = 'rndmroll.reminder_on';
const REMINDER_ASKED_KEY = 'rndmroll.reminder_asked';

/** What the phone allows: 'unavailable' when this build has no notification module. */
export type ReminderPermission = 'granted' | 'denied' | 'undetermined' | 'unavailable';

/** The notification's text and where a tap goes. Pure. */
export function reminderContent(): NotificationsModule.NotificationContentInput {
  return { title: REMINDER_TITLE, body: REMINDER_BODY, data: { url: REMINDER_ROUTE } };
}

/** Every day at 20:00 local time. Pure; `daily` is the module's DAILY trigger type. */
export function reminderTrigger(daily: NotificationsModule.SchedulableTriggerInputTypes.DAILY) {
  return { type: daily, hour: REMINDER_HOUR, minute: REMINDER_MINUTE };
}

/** True when a reminder should be scheduled. Pure. */
export function shouldSchedule(switchOn: boolean, permission: ReminderPermission): boolean {
  return switchOn && permission === 'granted';
}

/** What the switch shows: on only while it is on AND the phone allows notifications. Pure. */
export function switchShowsOn(switchOn: boolean, permission: ReminderPermission): boolean {
  return shouldSchedule(switchOn, permission);
}

/** The line under the switch: only when the person turned the phone's permission off. Pure. */
export function permissionLine(permission: ReminderPermission): string {
  return permission === 'denied' ? PERMISSION_OFF_MESSAGE : '';
}

/** The module's permission answer, reduced to the four states above. Pure. */
export function permissionFrom(answer: { granted: boolean; status: string }): ReminderPermission {
  if (answer.granted) return 'granted';
  if (answer.status === 'undetermined') return 'undetermined';
  return 'denied';
}

let notifications: typeof NotificationsModule | null | undefined;

/**
 * The notification module, loaded once on first use. Null when this build has no native module
 * (the dev client before its rebuild): loading it would throw, so the app goes on without it.
 */
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

/** True when the reminder can work on this build. */
export function reminderAvailable(): boolean {
  return loadNotifications() !== null;
}

async function readFlag(key: string): Promise<boolean> {
  try {
    return (await SecureStore.getItemAsync(key)) === 'true';
  } catch {
    return false;
  }
}

async function writeFlag(key: string, value: boolean): Promise<void> {
  try {
    await SecureStore.setItemAsync(key, value ? 'true' : 'false');
  } catch {
    // Swallowed: worst case the switch shows its old value next time.
  }
}

/** The switch value kept on the phone. Off until the person turns it on. */
export function readReminderOn(): Promise<boolean> {
  return readFlag(REMINDER_ON_KEY);
}

/** What the phone allows right now. Never throws. */
export async function getReminderPermission(): Promise<ReminderPermission> {
  const api = loadNotifications();
  if (api === null) return 'unavailable';
  try {
    return permissionFrom(await api.getPermissionsAsync());
  } catch {
    return 'unavailable';
  }
}

/** Shows the system prompt if it was never answered; answers what the phone allows afterwards. */
async function askPermission(): Promise<ReminderPermission> {
  const current = await getReminderPermission();
  if (current !== 'undetermined') return current;
  const api = loadNotifications();
  if (api === null) return 'unavailable';
  try {
    return permissionFrom(await api.requestPermissionsAsync());
  } catch {
    return 'unavailable';
  }
}

let rebuildChain: Promise<void> = Promise.resolve();

/**
 * Cancels every scheduled notification, then schedules the one reminder if the switch is on and
 * the phone allows it. Rebuilds run strictly one after another, so two never interleave into two
 * reminders. Never throws.
 */
export function rebuildReminder(): Promise<void> {
  const next = rebuildChain.then(async () => {
    const api = loadNotifications();
    if (api === null) return;
    try {
      await api.cancelAllScheduledNotificationsAsync();
      const [switchOn, permission] = await Promise.all([readReminderOn(), getReminderPermission()]);
      if (!shouldSchedule(switchOn, permission)) return;
      await api.scheduleNotificationAsync({
        identifier: REMINDER_ID,
        content: reminderContent(),
        trigger: reminderTrigger(api.SchedulableTriggerInputTypes.DAILY),
      });
    } catch {
      // Nothing scheduled this time; the next app start tries again.
    }
  });
  rebuildChain = next;
  return next;
}

/**
 * Turns the switch on or off and rebuilds the schedule. Turning it on asks for the phone's
 * permission first if it was never answered; without permission the switch stays off. Answers
 * what the switch shows now and the permission it was decided with.
 */
export async function setReminderOn(on: boolean): Promise<{ on: boolean; permission: ReminderPermission }> {
  if (!on) {
    await writeFlag(REMINDER_ON_KEY, false);
    await rebuildReminder();
    return { on: false, permission: await getReminderPermission() };
  }
  const permission = await askPermission();
  const allowed = permission === 'granted';
  await writeFlag(REMINDER_ON_KEY, allowed);
  await rebuildReminder();
  return { on: allowed, permission };
}

/**
 * The one-time ask at the beginning (developer, 2026-10-19: "can RNDMRoll send notifications" at
 * the start, not after the first post): on the first signed-in start, the phone's own prompt to
 * allow notifications. Allow turns the 8 PM reminder on; Don't Allow leaves it off. It is marked
 * asked first, so it never comes again whatever the answer; the switch on Settings >
 * Notifications is the way back. Skipped when this build has no notification module.
 */
export async function askAtStartOnce(): Promise<void> {
  if (!reminderAvailable()) return;
  if (await readFlag(REMINDER_ASKED_KEY)) return;
  await writeFlag(REMINDER_ASKED_KEY, true);
  const permission = await askPermission();
  if (permission === 'granted') await writeFlag(REMINDER_ON_KEY, true);
  await rebuildReminder();
}

/** The id of the last tap already handled, so a remount never opens the Spin tab twice for it. */
let handledResponseId: string | null = null;

/**
 * Signed-in app start: rebuilds the schedule, lets the reminder show while the app is open, and
 * opens the Spin tab when the reminder is tapped (also the tap that started the app). The schedule
 * is rebuilt again whenever the app comes back to the front, so a permission turned back on in the
 * phone's Settings takes effect at once.
 */
export function useReminderSetup(): void {
  const router = useRouter();

  useEffect(() => {
    const api = loadNotifications();
    if (api === null) return;

    // The first signed-in start asks to allow notifications (and then rebuilds); later starts
    // only rebuild.
    void askAtStartOnce().then(() => rebuildReminder());

    try {
      api.setNotificationHandler({
        handleNotification: async () => ({
          shouldShowBanner: true,
          shouldShowList: true,
          shouldPlaySound: false,
          shouldSetBadge: false,
        }),
      });
    } catch {
      // The reminder still shows when the app is closed.
    }

    function open(response: NotificationsModule.NotificationResponse | null) {
      if (response === null) return;
      const request = response.notification.request;
      if (request.identifier !== REMINDER_ID) return;
      const responseId = `${request.identifier}:${response.notification.date}`;
      if (handledResponseId === responseId) return;
      handledResponseId = responseId;
      router.navigate(REMINDER_ROUTE);
    }

    let alive = true;
    api
      .getLastNotificationResponseAsync()
      .then((response) => {
        if (alive) open(response);
      })
      .catch(() => {
        // No launch tap to follow.
      });

    let subscription: { remove: () => void } | null = null;
    try {
      subscription = api.addNotificationResponseReceivedListener(open);
    } catch {
      subscription = null;
    }

    const foreground = AppState.addEventListener('change', (next) => {
      if (next === 'active') void rebuildReminder();
    });

    return () => {
      alive = false;
      subscription?.remove();
      foreground.remove();
    };
  }, [router]);
}

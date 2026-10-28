import { useEffect } from 'react';
import { AppState } from 'react-native';
import { useRouter } from 'expo-router';
import * as SecureStore from 'expo-secure-store';
import type * as NotificationsModule from 'expo-notifications';
import { listenForTaps, loadNotifications, queueScheduleChange } from '@/lib/notifications/module';
import { POST_NOTE_TITLE, postNoteBody, postNoteDate } from '@/lib/notifications/postNote';
import { getAccessToken, useSession } from '@/lib/session/store';
import { useTodaySnapshot } from '@/lib/wheel/todayStore';

/**
 * The 8 PM reminder (Phase 6, 06-CONTEXT.md "Reminder" and 06-FOLLOW-OVERRIDES.md section 7): a
 * LOCAL phone notification at 8:00 PM in the phone's own time, every day, saying the spin is open.
 * The server is not involved, so no Apple push key is needed.
 *
 * The same switch also covers the morning note (lib/notifications/postNote.ts): for a spin made and
 * not yet posted or skipped, one note at 10 AM, two hours before a post counts as late. It is
 * scheduled from Today's `late_from` with the evenings, so a post or a skip (which clears
 * `late_from`) takes it away at the next rebuild.
 *
 * Where things live:
 *  - The switch value and the "asked once" mark are kept on the phone with expo-secure-store, as
 *    the paper says (not in lib/session/store.ts, which keeps only the session). They are plain
 *    flags, not credentials.
 *  - On every signed-in app start `useReminderSetup` (mounted by the signed-in layout) rebuilds the
 *    schedule: cancel everything, then schedule one if the switch is on and the phone allows it.
 *    Every rebuild runs one after another and the one notification has a fixed id, so the reminder
 *    can never be scheduled twice.
 *  - When a session ends (any log out, also an account deleted or disabled), the session store
 *    cancels every scheduled notification in the same queue as the rebuilds (module.ts), so a phone
 *    nobody is signed in on is never told "Your spin is open." The switch and the "asked once" mark
 *    stay as they are: the next person who signs in here is scheduled by the same rules, with no
 *    new ask.
 *  - The native module arrives with a dev-client rebuild. Until then (or on any build without it)
 *    the module is loaded lazily and every call is guarded: the reminder is simply unavailable, the
 *    switch shows off, there is no ask, and the app starts as before.
 */

/** 8:00 PM, in the phone's own time zone. */
export const REMINDER_HOUR = 20;
export const REMINDER_MINUTE = 0;

export const REMINDER_TITLE = "Spin o'clock";
export const REMINDER_BODY = 'Your wheel is ready when you are.';

/** Tapping the reminder opens the Spin tab. */
export const REMINDER_ROUTE = '/(app)/today';

/** The one scheduled notification's id: scheduling again replaces it instead of adding one. */
export const REMINDER_ID = 'rndmroll-daily-spin-reminder';

/** The morning note's id. It starts with the reminder's, so a tap opens the Spin tab and push ignores it. */
export const POST_NOTE_ID = `${REMINDER_ID}-post`;

/** The switch's label on Settings > Notifications: the 8 PM reminder and the morning note. */
export const REMINDER_SWITCH_LABEL = 'Remind me to spin and post';

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

/** How many evenings ahead the reminder is set one by one when the next opening is known. */
export const REMINDER_DAYS_AHEAD = 30;

/**
 * The evenings the reminder fires on, at 8:00 PM phone time, when the next opening is known
 * (Today's next_open_at): every 8:00 PM from today on that is still ahead and not before that
 * opening. So a person who already made today's spin (a first spin in the morning, developer
 * 2026-10-19) is not told "Your spin is open." that same evening. Pure.
 */
export function reminderDates(nowMs: number, nextOpenMs: number, days = REMINDER_DAYS_AHEAD): Date[] {
  const out: Date[] = [];
  const start = new Date(nowMs);
  for (let i = 0; out.length < days && i < days + 2; i += 1) {
    const at = new Date(start.getFullYear(), start.getMonth(), start.getDate() + i, REMINDER_HOUR, REMINDER_MINUTE, 0, 0);
    // A minute of slack: the server's 8 PM and the phone's can differ by a little.
    if (at.getTime() > nowMs && at.getTime() >= nextOpenMs - 60_000) out.push(at);
  }
  return out;
}

/** True for any of the reminder's own scheduled notifications. Pure. */
export function isReminderId(id: string): boolean {
  return id === REMINDER_ID || id.startsWith(`${REMINDER_ID}-`);
}

/** The next opening the server last gave (ms), or null before the first Today answer. */
let nextOpenMs: number | null = null;

/**
 * Today's next opening, from every Today answer: the schedule is rebuilt when it moves, so the
 * reminder skips an evening whose spin was already made.
 */
export function noteNextOpen(iso: string): void {
  const ms = Date.parse(iso);
  if (!Number.isFinite(ms) || ms === nextOpenMs) return;
  nextOpenMs = ms;
  void rebuildReminder();
}

/** The spin waiting for a post, from the last Today answer: when it turns late and its category. */
let postNote: { lateFromMs: number; category: string } | null = null;

/**
 * Today's `late_from` and the shown spin's category, from every Today answer. The schedule is
 * rebuilt when they change, so the morning note follows a new spin, and a post or a skip (no
 * `late_from`) takes it away.
 */
export function notePostNote(lateFromIso: string | null, categoryName: string | null): void {
  const ms = lateFromIso === null ? NaN : Date.parse(lateFromIso);
  const next = Number.isFinite(ms) && categoryName !== null ? { lateFromMs: ms, category: categoryName } : null;
  if (next?.lateFromMs === postNote?.lateFromMs && next?.category === postNote?.category) return;
  postNote = next;
  void rebuildReminder();
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

/**
 * Cancels every scheduled notification, then schedules the one reminder if the switch is on, the
 * phone allows it and someone is signed in. Rebuilds run strictly one after another, in the queue
 * that also holds the cancel when a session ends (module.ts), so two never interleave into two
 * reminders. Never throws.
 */
export function rebuildReminder(): Promise<void> {
  return queueScheduleChange(async (api) => {
    try {
      await api.cancelAllScheduledNotificationsAsync();
      const [switchOn, permission] = await Promise.all([readReminderOn(), getReminderPermission()]);
      if (!shouldSchedule(switchOn, permission)) return;
      // Only while a session is signed in on this phone (its access token is in memory). A rebuild
      // the signed-in pages queued just before a log out (one held behind the first-start
      // permission prompt, say) runs after that log out's cancel, and must not schedule again.
      if (getAccessToken() === null) return;
      // The morning note, for a spin made and not yet posted or skipped.
      const noteAt = postNote === null ? null : postNoteDate(Date.now(), postNote.lateFromMs);
      if (postNote !== null && noteAt !== null) {
        await api.scheduleNotificationAsync({
          identifier: POST_NOTE_ID,
          content: { title: POST_NOTE_TITLE, body: postNoteBody(postNote.category), data: { url: REMINDER_ROUTE } },
          trigger: { type: api.SchedulableTriggerInputTypes.DATE, date: noteAt },
        });
      }
      if (nextOpenMs === null) {
        // Not known yet: every evening, until the first Today answer narrows it.
        await api.scheduleNotificationAsync({
          identifier: REMINDER_ID,
          content: reminderContent(),
          trigger: reminderTrigger(api.SchedulableTriggerInputTypes.DAILY),
        });
        return;
      }
      // Known: each evening on its own, from the next opening on (rebuilt on every start).
      const dates = reminderDates(Date.now(), nextOpenMs);
      for (let i = 0; i < dates.length; i += 1) {
        await api.scheduleNotificationAsync({
          identifier: `${REMINDER_ID}-${i}`,
          content: reminderContent(),
          trigger: { type: api.SchedulableTriggerInputTypes.DATE, date: dates[i] },
        });
      }
    } catch {
      // Nothing scheduled this time; the next app start tries again.
    }
  });
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
  const { user } = useSession();
  const snapshot = useTodaySnapshot(user?.id ?? null);
  const nextOpenAt = snapshot.today?.next_open_at ?? null;
  const lateFrom = snapshot.today?.late_from ?? null;
  const waitingCategory = snapshot.today?.spin?.category.name ?? null;

  // Every Today answer: the reminder starts from the next opening, so an evening whose spin was
  // already made (a first spin in the morning) gets no reminder.
  useEffect(() => {
    if (nextOpenAt !== null) noteNextOpen(nextOpenAt);
  }, [nextOpenAt]);

  // Every Today answer: the morning note follows the spin waiting for a post, if there is one. No
  // Today yet (a start, or another person signing in on this phone) clears it, so a note never
  // carries the last person's spin.
  useEffect(() => {
    notePostNote(lateFrom, waitingCategory);
  }, [lateFrom, waitingCategory]);

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
      if (!isReminderId(request.identifier)) return;
      const responseId = `${request.identifier}:${response.notification.date}`;
      if (handledResponseId === responseId) return;
      handledResponseId = responseId;
      router.navigate(REMINDER_ROUTE);
    }

    const stopTaps = listenForTaps(api, open);

    const foreground = AppState.addEventListener('change', (next) => {
      if (next === 'active') void rebuildReminder();
    });

    return () => {
      stopTaps();
      foreground.remove();
    };
  }, [router]);
}

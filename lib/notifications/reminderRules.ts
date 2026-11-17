/**
 * The 8 PM reminder's pure rules (lib/notifications/reminder.ts schedules it): when it fires, its
 * id, and what the switch on Settings > Notifications shows. Pure: no imports, so a node script
 * can run them (scripts/check-push.ts). reminder.ts exports all of them as before.
 */

/** 8:00 PM, in the phone's own time zone. */
export const REMINDER_HOUR = 20;
export const REMINDER_MINUTE = 0;

/** The one scheduled notification's id: scheduling again replaces it instead of adding one. */
export const REMINDER_ID = 'rndmroll-daily-spin-reminder';

/** How many evenings ahead the reminder is set one by one when the next opening is known. */
export const REMINDER_DAYS_AHEAD = 30;

/** The one line under the switch while the phone's own permission is off. */
export const PERMISSION_OFF_MESSAGE = 'Turn on notifications for RNDMRoll in Settings.';

/** What the phone allows: 'unavailable' when this build has no notification module. */
export type ReminderPermission = 'granted' | 'denied' | 'undetermined' | 'unavailable';

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

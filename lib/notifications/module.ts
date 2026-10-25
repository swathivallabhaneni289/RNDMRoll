import type * as NotificationsModule from 'expo-notifications';

/**
 * The phone's notification module, shared by the 8 PM reminder (reminder.ts) and the phone pushes
 * (push.ts, pushToken.ts). It imports nothing from the app, so the session store can reach it
 * (through pushToken.ts, and directly for the cancel when a session ends) without an import loop.
 */

let notifications: typeof NotificationsModule | null | undefined;

/**
 * The notification module, loaded once on first use. Null when this build has no native module
 * (the dev client before its rebuild): loading it would throw, so the app goes on without it.
 */
export function loadNotifications(): typeof NotificationsModule | null {
  if (notifications !== undefined) return notifications;
  try {
    // eslint-disable-next-line @typescript-eslint/no-require-imports
    notifications = require('expo-notifications') as typeof NotificationsModule;
  } catch {
    notifications = null;
  }
  return notifications;
}

/**
 * Every change to the phone's scheduled notifications, one after another: the 8 PM reminder's
 * rebuilds (reminder.ts) and the cancel when a session ends (cancelScheduledNotifications). The
 * queue lives here and not in reminder.ts because the session store, which cancels, cannot import
 * reminder.ts (it reads the session) without an import loop. A change starts only once every change
 * queued before it has finished, so the cancel never lands in the middle of a rebuild, and the
 * rebuild of whoever signs in next is queued after the cancel and is never undone by it.
 */
let scheduleQueue: Promise<void> = Promise.resolve();

/**
 * Runs change with the module once every change queued before it has finished. Never throws, so one
 * that fails never holds up the ones after it; on a build without the module it does nothing.
 */
export function queueScheduleChange(change: (api: typeof NotificationsModule) => Promise<void>): Promise<void> {
  const next = scheduleQueue.then(async () => {
    const api = loadNotifications();
    if (api === null) return;
    try {
      await change(api);
    } catch {
      // Left as it is; the next change starts from whatever the phone holds.
    }
  });
  scheduleQueue = next;
  return next;
}

/**
 * The session ended (Log out, a deleted or disabled account, a refresh the server refused): every
 * scheduled notification is cancelled, so this phone stops saying "Your spin is open." for an
 * account that is no longer signed in on it. Only the 8 PM reminder schedules any. It asks for no
 * permission (none is needed to cancel) and leaves the reminder's switch and its "asked once" mark
 * as they are, so the next person who signs in here gets the reminder by the same rules as before.
 * Never throws; on a build without the module it does nothing.
 */
export function cancelScheduledNotifications(): Promise<void> {
  return queueScheduleChange((api) => api.cancelAllScheduledNotificationsAsync());
}

/**
 * Hands taps to open: the tap that started the app (null when there was none), then every later
 * tap, until the returned function is called. Each caller picks its own taps and remembers its own
 * last one.
 */
export function listenForTaps(
  api: typeof NotificationsModule,
  open: (response: NotificationsModule.NotificationResponse | null) => void
): () => void {
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
  return () => {
    alive = false;
    subscription?.remove();
  };
}

import type * as NotificationsModule from 'expo-notifications';

/**
 * The phone's notification module, shared by the 8 PM reminder (reminder.ts) and the phone pushes
 * (push.ts, pushToken.ts). It imports nothing from the app, so the session store can reach it
 * through pushToken.ts without an import loop.
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

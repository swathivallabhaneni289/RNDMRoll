import { useEffect } from 'react';
import { AppState } from 'react-native';
import { useRouter } from 'expo-router';
import type * as NotificationsModule from 'expo-notifications';
import { useSession } from '@/lib/session/store';
import { isReminderId } from '@/lib/notifications/reminder';
import { pushTarget } from '@/lib/notifications/pushTarget';
import { registerPushToken } from '@/lib/notifications/pushToken';

/**
 * Phone pushes while signed in (docs/PUSH-SETUP.md): sends this phone's push token to the server
 * (lib/notifications/pushToken.ts) and opens the right page when a push is tapped. Mounted by the
 * signed-in layout next to the 8 PM reminder, whose own taps it leaves alone. How a notification
 * shows while the app is open is set once, by useReminderSetup.
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

/** The last push tap already followed, so a remount never opens its page twice. */
let handledResponseId: string | null = null;

export function usePushSetup(): void {
  const router = useRouter();
  const { user } = useSession();
  const userId = user?.id ?? null;

  // The token: now, and every time the app comes back to the front (the permission may have just
  // been given in the start-up ask or in the phone's Settings).
  useEffect(() => {
    if (userId === null) return;
    void registerPushToken(userId);
    const foreground = AppState.addEventListener('change', (next) => {
      if (next === 'active') void registerPushToken(userId);
    });
    return () => foreground.remove();
  }, [userId]);

  // A tap on a push, also the tap that started the app.
  useEffect(() => {
    const api = loadNotifications();
    if (api === null) return;

    function open(response: NotificationsModule.NotificationResponse | null) {
      if (response === null) return;
      const request = response.notification.request;
      if (isReminderId(request.identifier)) return;
      const target = pushTarget(request.content.data);
      if (target === null) return;
      const responseId = `${request.identifier}:${response.notification.date}`;
      if (handledResponseId === responseId) return;
      handledResponseId = responseId;
      router.push(target);
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
    return () => {
      alive = false;
      subscription?.remove();
    };
  }, [router]);
}

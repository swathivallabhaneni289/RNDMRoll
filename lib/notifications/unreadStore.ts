import { useCallback, useSyncExternalStore } from 'react';
import { useFocusEffect } from 'expo-router';
import { getNotificationSummary } from '@/lib/api/notifications';
import { useSession } from '@/lib/session/store';

/**
 * The dot on the bell (Phase 5). One small module store read with `useSyncExternalStore`, kept
 * for one signed-in person: another account's value is never shown. The Spin page and the Board
 * ask `GET /notifications/summary` each time they come into focus; opening Notifications marks
 * everything seen and puts the dot out. A failed ask keeps what is shown.
 */

type UnreadState = { userId: string | null; unread: boolean };

let state: UnreadState = { userId: null, unread: false };
const listeners = new Set<() => void>();

function emit(): void {
  for (const listener of Array.from(listeners)) listener();
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

function getSnapshot(): UnreadState {
  return state;
}

/** Sets the dot for this person. */
export function setUnread(userId: string, unread: boolean): void {
  if (state.userId === userId && state.unread === unread) return;
  state = { userId, unread };
  emit();
}

let asking: Promise<void> | null = null;

/** Asks the server once at a time. A failure keeps the dot as it is. */
export function refreshUnread(userId: string): Promise<void> {
  if (asking) return asking;
  asking = getNotificationSummary()
    .then((unread) => setUnread(userId, unread))
    .catch(() => undefined)
    .finally(() => {
      asking = null;
    });
  return asking;
}

/** The dot for the signed-in person, asked again each time the screen that shows it is focused. */
export function useUnread(): boolean {
  const { user } = useSession();
  const userId = user?.id ?? null;
  const snapshot = useSyncExternalStore(subscribe, getSnapshot, getSnapshot);

  useFocusEffect(
    useCallback(() => {
      if (userId !== null) void refreshUnread(userId);
      return undefined;
    }, [userId])
  );

  return userId !== null && snapshot.userId === userId && snapshot.unread;
}

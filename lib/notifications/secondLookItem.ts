import type { RemovalKind } from '@/lib/notifications/moderation';

/**
 * The removed post or comment the bell opened the second look's page for (developer, 2026-11-19).
 * The bell keeps it here just before the push, so the route carries only the removal's id and the
 * page shows the item's words at once. A page opened without it reads the bell's list instead.
 * One item at a time. The page also says here when a send ends after the person left it, so the
 * bell reads its list again (secondLookSent).
 */
export type SecondLookItem = {
  /** The removal's full id, the bell item's `removal`. */
  removal: string;
  kind: RemovalKind;
  /** The post's title or the comment's words, as the bell has them; '' for a photo only. */
  text: string;
};

let chosen: SecondLookItem | null = null;

/** Kept by the bell just before it opens the page. */
export function chooseSecondLook(item: SecondLookItem): void {
  chosen = item;
}

/** The item the bell kept for this removal, or null. */
export function chosenSecondLook(removal: string): SecondLookItem | null {
  return chosen !== null && chosen.removal === removal ? chosen : null;
}

const sentListeners = new Set<() => void>();

/**
 * Said by the page when a second look is sent after the person left it (Back while it sent): the
 * bell read its list as they came back, before the request was in, so it reads it again and the
 * line says "Second look asked." A page still showing goes back instead, and the bell reads its
 * list then.
 */
export function secondLookSent(): void {
  sentListeners.forEach((listener) => listener());
}

/** The bell listens while it is open; the returned function stops it. */
export function onSecondLookSent(listener: () => void): () => void {
  sentListeners.add(listener);
  return () => {
    sentListeners.delete(listener);
  };
}

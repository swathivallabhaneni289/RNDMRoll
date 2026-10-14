import type { useRouter } from 'expo-router';

/**
 * 'Back to Today' from a page that sits one or two screens above Today (the entry page, edit
 * entry): close everything above the Home stack's first screen. If nothing is above it (a
 * deep link), put Today in place instead.
 */
export function goToToday(router: ReturnType<typeof useRouter>): void {
  if (router.canDismiss()) router.dismissAll();
  else router.replace('/(app)/today');
}

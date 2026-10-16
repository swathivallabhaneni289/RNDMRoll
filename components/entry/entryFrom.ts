import type { useRouter } from 'expo-router';
import { goToToday } from '@/components/entry/goToToday';

/**
 * Where the entry page was opened from (04-UI-SPEC 3.7 and 4.4). The same page and the same edit
 * page sit in three stacks (Spin, Profile and Board; the Diary tab was removed 2026-10-16), each with its own thin
 * routes, so a push never switches tabs. `from` picks the Back name, the gone-state button, the Edit route and the stack
 * root a fallback goes to. Each stack's thin route fixes its own value.
 */
export type EntryFrom = 'today' | 'profile' | 'board';

type Root = '/(app)/today' | '/(app)/profile' | '/(app)/board';

const PLACES: Record<EntryFrom, { name: string; root: Root }> = {
  today: { name: 'Today', root: '/(app)/today' },
  profile: { name: 'Profile', root: '/(app)/profile' },
  board: { name: 'Board', root: '/(app)/board' },
};

/** The name on the Back row: 'Today', 'Profile' or 'Board'. */
export function fromName(from: EntryFrom): string {
  return PLACES[from].name;
}

/** The stack's first page, for a fallback when there is nothing to go back to (a deep link). */
export function fromRoot(from: EntryFrom): Root {
  return PLACES[from].root;
}

/** The edit page inside the same stack. */
export function editEntryPath(
  from: EntryFrom
): '/(app)/today/edit-entry' | '/(app)/profile/edit-entry' | '/(app)/board/edit-entry' {
  if (from === 'profile') return '/(app)/profile/edit-entry';
  if (from === 'board') return '/(app)/board/edit-entry';
  return '/(app)/today/edit-entry';
}

/**
 * 'Back to Today', 'Back to Profile' or 'Back to Board' from a gone entry: close everything above
 * the stack's first page. With nothing above it (a deep link), put that first page in place.
 */
export function goToStart(router: ReturnType<typeof useRouter>, from: EntryFrom): void {
  if (from === 'today') {
    goToToday(router);
    return;
  }
  if (router.canDismiss()) router.dismissAll();
  else router.replace(fromRoot(from));
}

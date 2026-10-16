import type { useRouter } from 'expo-router';
import { goToToday } from '@/components/entry/goToToday';

/**
 * Where the entry page was opened from (04-UI-SPEC 3.7 and 4.4). The same page and the same edit
 * page sit in three stacks (Home, Diary, Profile), each with its own thin routes, so a push never
 * switches tabs. `from` picks the Back name, the gone-state button, the Edit route and the stack
 * root a fallback goes to. Each stack's thin route fixes its own value.
 */
export type EntryFrom = 'today' | 'diary' | 'profile';

const PLACES: Record<EntryFrom, { name: string; root: '/(app)/today' | '/(app)/diary' | '/(app)/profile' }> = {
  today: { name: 'Today', root: '/(app)/today' },
  diary: { name: 'Diary', root: '/(app)/diary' },
  profile: { name: 'Profile', root: '/(app)/profile' },
};

/** The name on the Back row: 'Today', 'Diary' or 'Profile'. */
export function fromName(from: EntryFrom): string {
  return PLACES[from].name;
}

/** The stack's first page, for a fallback when there is nothing to go back to (a deep link). */
export function fromRoot(from: EntryFrom): '/(app)/today' | '/(app)/diary' | '/(app)/profile' {
  return PLACES[from].root;
}

/** The edit page inside the same stack. */
export function editEntryPath(from: EntryFrom): '/(app)/today/edit-entry' | '/(app)/diary/edit-entry' | '/(app)/profile/edit-entry' {
  if (from === 'diary') return '/(app)/diary/edit-entry';
  if (from === 'profile') return '/(app)/profile/edit-entry';
  return '/(app)/today/edit-entry';
}

/**
 * 'Back to Today', 'Back to Diary' or 'Back to Profile' from a gone entry: close everything above
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

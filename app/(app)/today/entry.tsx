import { useLocalSearchParams } from 'expo-router';
import { EntryView } from '@/components/entry/EntryView';

/**
 * The entry page (03-UI-SPEC section 9). A thin route: it reads the entry id and renders `EntryView`
 * with `from` fixed to 'today' (the Diary and Profile stacks have their own thin routes). The
 * Tabs layout hides the bottom bar while this route is focused (its name is in the hide list), and
 * the pinned Edit and Delete footer takes the bar's place. An id that is not a UUID shows 'This
 * entry isn't here any more.' and is never sent to the server.
 */
export default function EntryRoute() {
  const params = useLocalSearchParams<{ id?: string | string[]; via?: string | string[] }>();
  const id = Array.isArray(params.id) ? params.id[0] : params.id;
  const via = Array.isArray(params.via) ? params.via[0] : params.via;
  // Opened from a reaction or comment in Notifications (Phase 5): the Back row names that page.
  return <EntryView id={id ?? ''} from="today" backLabel={via === 'notifications' ? 'Notifications' : undefined} />;
}

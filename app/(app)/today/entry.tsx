import { useLocalSearchParams } from 'expo-router';
import { EntryView } from '@/components/entry/EntryView';

/**
 * The entry page (03-UI-SPEC section 9). A thin route: it reads the entry id and `from` (the Back
 * label: 'today' now, 'diary' from the Phase 4 diary) and renders the `EntryView` component. The
 * Tabs layout hides the bottom bar while this route is focused (its name is in the hide list), and
 * the pinned Edit and Delete footer takes the bar's place. An id that is not a UUID shows 'This
 * entry isn't here any more.' and is never sent to the server.
 */
export default function EntryRoute() {
  const params = useLocalSearchParams<{ id?: string | string[]; from?: string | string[] }>();
  const id = Array.isArray(params.id) ? params.id[0] : params.id;
  const from = Array.isArray(params.from) ? params.from[0] : params.from;
  return <EntryView id={id ?? ''} from={from === 'diary' ? 'diary' : 'today'} />;
}

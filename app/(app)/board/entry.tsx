import { useLocalSearchParams } from 'expo-router';
import { EntryView } from '@/components/entry/EntryView';

/**
 * Your own entry page opened from the Board (your card) or from the Board's Notifications: Back
 * reads 'Board' (or 'Notifications') and Edit stays in this stack.
 */
export default function BoardEntryRoute() {
  const params = useLocalSearchParams<{ id?: string | string[]; via?: string | string[] }>();
  const id = Array.isArray(params.id) ? params.id[0] : params.id;
  const via = Array.isArray(params.via) ? params.via[0] : params.via;
  return <EntryView id={id ?? ''} from="board" backLabel={via === 'notifications' ? 'Notifications' : undefined} />;
}

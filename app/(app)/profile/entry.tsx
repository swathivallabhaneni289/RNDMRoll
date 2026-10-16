import { useLocalSearchParams } from 'expo-router';
import { EntryView } from '@/components/entry/EntryView';

/** The entry page opened from 'View comments' in the Profile posts list: Back reads 'Profile'. */
export default function ProfileEntryRoute() {
  const params = useLocalSearchParams<{ id?: string | string[] }>();
  const id = Array.isArray(params.id) ? params.id[0] : params.id;
  return <EntryView id={id ?? ''} from="profile" />;
}

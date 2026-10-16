import { useLocalSearchParams } from 'expo-router';
import { EditEntryScreen } from '@/components/entry/EditEntryScreen';

/** Edit entry opened from the Profile stack's entry page. */
export default function ProfileEditEntryRoute() {
  const params = useLocalSearchParams<{ id?: string | string[] }>();
  const id = Array.isArray(params.id) ? params.id[0] : params.id;
  return <EditEntryScreen id={id ?? ''} from="profile" />;
}

import { useLocalSearchParams } from 'expo-router';
import { EditEntryScreen } from '@/components/entry/EditEntryScreen';

/** Edit entry opened from the Diary's entry page. */
export default function DiaryEditEntryRoute() {
  const params = useLocalSearchParams<{ id?: string | string[] }>();
  const id = Array.isArray(params.id) ? params.id[0] : params.id;
  return <EditEntryScreen id={id ?? ''} from="diary" />;
}

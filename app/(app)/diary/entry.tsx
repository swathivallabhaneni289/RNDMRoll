import { useLocalSearchParams } from 'expo-router';
import { EntryView } from '@/components/entry/EntryView';

/** The entry page opened from the Diary: Back reads 'Diary' and Edit stays in this stack. */
export default function DiaryEntryRoute() {
  const params = useLocalSearchParams<{ id?: string | string[] }>();
  const id = Array.isArray(params.id) ? params.id[0] : params.id;
  return <EntryView id={id ?? ''} from="diary" />;
}

import { useLocalSearchParams } from 'expo-router';
import { EditEntryScreen } from '@/components/entry/EditEntryScreen';

/**
 * Edit entry under Today (03-UI-SPEC section 10). A thin route: it reads the entry id and renders
 * `EditEntryScreen`, which the Diary and Profile stacks mount too. The Tabs layout hides the bar
 * while this route is focused.
 */
export default function EditEntryRoute() {
  const params = useLocalSearchParams<{ id?: string | string[] }>();
  const id = Array.isArray(params.id) ? params.id[0] : params.id;
  return <EditEntryScreen id={id ?? ''} from="today" />;
}

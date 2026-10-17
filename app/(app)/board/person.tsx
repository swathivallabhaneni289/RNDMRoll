import { useLocalSearchParams } from 'expo-router';
import { PersonScreen } from '@/components/people/PersonScreen';

/** Someone's profile page, opened by a tap on their photo or name in this tab. */
export default function BoardPersonRoute() {
  const params = useLocalSearchParams<{ id?: string | string[] }>();
  const id = Array.isArray(params.id) ? params.id[0] : params.id;
  return <PersonScreen id={id ?? ''} />;
}

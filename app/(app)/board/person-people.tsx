import { useLocalSearchParams } from 'expo-router';
import { PersonListScreen } from '@/components/people/PersonListScreen';

/** Someone's Followers and Following, opened from those figures on their profile page in this tab. */
export default function BoardPersonPeopleRoute() {
  const params = useLocalSearchParams<{ id?: string | string[]; username?: string | string[]; show?: string | string[] }>();
  const one = (v?: string | string[]) => (Array.isArray(v) ? v[0] : v) ?? '';
  return <PersonListScreen id={one(params.id)} username={one(params.username)} show={one(params.show) === 'following' ? 'following' : 'followers'} />;
}

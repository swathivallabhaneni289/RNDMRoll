import { useLocalSearchParams } from 'expo-router';
import { PeopleScreen } from '@/components/people/PeopleScreen';

/**
 * People, opened from the Followers and Following figures (`show` picks the tab) and from a follow
 * notification: Back reads 'Profile'.
 */
export default function ProfilePeopleRoute() {
  const { show } = useLocalSearchParams<{ show?: string }>();
  return <PeopleScreen from="profile" show={show === 'following' ? 'following' : 'followers'} />;
}

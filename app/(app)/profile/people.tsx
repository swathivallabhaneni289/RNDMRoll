import { PeopleScreen } from '@/components/people/PeopleScreen';

/** People, opened from the Followers figure and from a follow notification: Back reads 'Profile'. */
export default function ProfilePeopleRoute() {
  return <PeopleScreen from="profile" />;
}

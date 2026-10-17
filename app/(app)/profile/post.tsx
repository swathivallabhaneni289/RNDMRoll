import { useLocalSearchParams } from 'expo-router';
import { PostView } from '@/components/board/PostView';

/** Someone else's post opened from their profile page in the Profile tab: Back reads their username. No bottom bar. */
export default function ProfilePostRoute() {
  const params = useLocalSearchParams<{ id?: string | string[]; back?: string | string[] }>();
  const id = Array.isArray(params.id) ? params.id[0] : params.id;
  const back = Array.isArray(params.back) ? params.back[0] : params.back;
  return <PostView id={id ?? ''} backName={back || 'Profile'} />;
}

import { useLocalSearchParams } from 'expo-router';
import { PostView } from '@/components/board/PostView';

/**
 * Someone else's post opened from the Spin page's Notifications (a reply to your comment): Back
 * reads 'Notifications'. No bottom bar.
 */
export default function TodayPostRoute() {
  const params = useLocalSearchParams<{ id?: string | string[]; back?: string | string[] }>();
  const id = Array.isArray(params.id) ? params.id[0] : params.id;
  // From someone's profile page, Back reads their username.
  const back = Array.isArray(params.back) ? params.back[0] : params.back;
  return <PostView id={id ?? ''} backName={back || 'Notifications'} place="today" />;
}

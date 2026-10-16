import { useLocalSearchParams } from 'expo-router';
import { PostView } from '@/components/board/PostView';

/**
 * Someone else's post opened from the Spin page's Notifications (a reply to your comment): Back
 * reads 'Notifications'. No bottom bar.
 */
export default function TodayPostRoute() {
  const params = useLocalSearchParams<{ id?: string | string[] }>();
  const id = Array.isArray(params.id) ? params.id[0] : params.id;
  return <PostView id={id ?? ''} backName="Notifications" place="today" />;
}

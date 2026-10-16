import { useLocalSearchParams } from 'expo-router';
import { PostsFeed } from '@/components/profile/PostsFeed';

/** Your posts as one scrolling list, opened from a picture in the Profile grid. */
export default function ProfilePostsRoute() {
  const params = useLocalSearchParams<{ start?: string | string[] }>();
  const start = Array.isArray(params.start) ? params.start[0] : params.start;
  return <PostsFeed startId={start ?? ''} />;
}

import { useLocalSearchParams } from 'expo-router';
import { PostView } from '@/components/board/PostView';

/** Someone else's post, opened from the Board: Back reads 'Board'. No bottom bar. */
export default function BoardPostRoute() {
  const params = useLocalSearchParams<{ id?: string | string[] }>();
  const id = Array.isArray(params.id) ? params.id[0] : params.id;
  return <PostView id={id ?? ''} backName="Board" />;
}

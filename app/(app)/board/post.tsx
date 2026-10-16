import { useLocalSearchParams } from 'expo-router';
import { PostView } from '@/components/board/PostView';

/**
 * Someone else's post, opened from the Board (Back reads 'Board') or from the Board's
 * Notifications (a reply to your comment; Back reads 'Notifications'). No bottom bar.
 */
export default function BoardPostRoute() {
  const params = useLocalSearchParams<{ id?: string | string[]; via?: string | string[] }>();
  const id = Array.isArray(params.id) ? params.id[0] : params.id;
  const via = Array.isArray(params.via) ? params.via[0] : params.via;
  return <PostView id={id ?? ''} backName={via === 'notifications' ? 'Notifications' : 'Board'} />;
}

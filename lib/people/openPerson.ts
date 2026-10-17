import { useRouter, useSegments } from 'expo-router';
import { useSession } from '@/lib/session/store';

/** The tab stacks that have a person page. */
type Stack = 'today' | 'board' | 'profile';

function stackOf(segments: string[]): Stack {
  const tab = segments[1];
  return tab === 'today' || tab === 'board' ? tab : 'profile';
}

/**
 * Opens a person's page from anywhere a photo or a name is shown, like Instagram: in the stack the
 * tap came from, so Back returns there. Your own photo or name opens your Profile tab instead.
 */
export function useOpenPerson() {
  const router = useRouter();
  const segments = useSegments() as string[];
  const { user } = useSession();
  return (userId: string) => {
    if (userId === '') return;
    if (userId === user?.id) {
      router.navigate('/(app)/profile');
      return;
    }
    router.push({ pathname: `/(app)/${stackOf(segments)}/person`, params: { id: userId } });
  };
}

/** The post page in the same stack as the person page it was opened from. */
export function postPathIn(segments: string[]): '/(app)/today/post' | '/(app)/board/post' | '/(app)/profile/post' {
  const stack = stackOf(segments);
  return stack === 'today' ? '/(app)/today/post' : stack === 'board' ? '/(app)/board/post' : '/(app)/profile/post';
}

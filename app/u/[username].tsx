import { useEffect } from 'react';
import { View } from 'react-native';
import { useLocalSearchParams, useRouter } from 'expo-router';
import { lookupPerson } from '@/lib/api/social';
import { useSession } from '@/lib/session/store';
import { color } from '@/lib/theme/tokens';

/**
 * An invite link opened in the app (rndmroll://u/{username}, the button on the invite page): the
 * person's profile on the Board tab, ready to Follow. Your own link opens your Profile. Someone
 * not signed in goes to the start (sign up or log in) first; a name that is not found, or a
 * block, opens Search instead. Nothing is drawn here but the page colour while it decides.
 */
export default function InviteLinkRoute() {
  const router = useRouter();
  const params = useLocalSearchParams<{ username?: string | string[] }>();
  const username = (Array.isArray(params.username) ? params.username[0] : params.username) ?? '';
  const { status, user } = useSession();

  useEffect(() => {
    if (status === 'loading') return;
    if (status !== 'authenticated' || user?.onboarding_complete !== true) {
      router.replace('/');
      return;
    }
    if (username.toLowerCase() === (user.username ?? '').toLowerCase()) {
      router.replace('/(app)/profile');
      return;
    }
    let alive = true;
    lookupPerson(username)
      .then((found) => {
        if (!alive) return;
        router.replace('/(app)/board');
        if (found !== null) router.push({ pathname: '/(app)/board/person', params: { id: found.person.id } });
        else router.push('/(app)/board/search');
      })
      .catch(() => {
        if (alive) router.replace('/(app)/board');
      });
    return () => {
      alive = false;
    };
  }, [status, user, username, router]);

  return <View style={{ flex: 1, backgroundColor: color.dominant }} />;
}

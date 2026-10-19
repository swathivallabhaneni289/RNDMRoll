import { useEffect, useState } from 'react';
import { View } from 'react-native';
import { useLocalSearchParams, useRouter } from 'expo-router';
import { PersonScreen } from '@/components/people/PersonScreen';
import { lookupPerson } from '@/lib/api/social';
import { useSession } from '@/lib/session/store';
import { color } from '@/lib/theme/tokens';

/**
 * Someone's profile page, opened by a tap on their photo or name in this tab, or by an invite
 * link (`username` in place of `id`, see app/+native-intent.tsx). Your own link opens your Profile.
 */
export default function BoardPersonRoute() {
  const router = useRouter();
  const { user } = useSession();
  const params = useLocalSearchParams<{ id?: string | string[]; username?: string | string[] }>();
  const given = Array.isArray(params.id) ? params.id[0] : params.id;
  const username = Array.isArray(params.username) ? params.username[0] : params.username;
  const [id, setId] = useState<string | null>(given ?? null);

  useEffect(() => {
    if (given || !username) return;
    if (username === (user?.username ?? '').toLowerCase()) {
      router.replace('/(app)/profile');
      return;
    }
    let alive = true;
    lookupPerson(username)
      .then((found) => {
        if (alive) setId(found?.person.id ?? '');
      })
      .catch(() => {
        if (alive) setId('');
      });
    return () => {
      alive = false;
    };
  }, [given, username, user, router]);

  if (id === null) return <View style={{ flex: 1, backgroundColor: color.dominant }} />;
  return <PersonScreen id={id} />;
}

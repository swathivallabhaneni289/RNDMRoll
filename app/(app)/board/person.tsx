import { useEffect, useState, type ReactNode } from 'react';
import { Pressable, View } from 'react-native';
import { useLocalSearchParams, useRouter } from 'expo-router';
import { Ionicons } from '@expo/vector-icons';
import { SafeAreaView } from 'react-native-safe-area-context';
import { PersonScreen } from '@/components/people/PersonScreen';
import { StatusMessage } from '@/components/today/StatusMessage';
import { AppText } from '@/components/ui/AppText';
import { ApiError } from '@/lib/api/client';
import { lookupPerson } from '@/lib/api/social';
import { useSession } from '@/lib/session/store';
import { color, minTouchTarget, space } from '@/lib/theme/tokens';

/** A link whose username leads to no one: the same line as a profile that is gone. */
const GONE_LINE = "This account isn't available.";

/** Where the lookup of an invite link's username stands. */
type Lookup =
  | { state: 'looking' }
  | { state: 'found'; id: string }
  | { state: 'gone' }
  | { state: 'failed'; failure: 'network' | 'generic'; retrying: boolean };

/** The Back row of a profile page with one thing under it, while there is no profile to show. */
function LinkPage({ children }: { children: ReactNode }) {
  const router = useRouter();

  function goBack() {
    if (router.canGoBack()) router.back();
    else router.replace('/(app)/board');
  }

  return (
    <SafeAreaView edges={['top', 'left', 'right']} style={{ flex: 1, backgroundColor: color.dominant }}>
      <View style={{ height: minTouchTarget, paddingHorizontal: space.lg, flexDirection: 'row', alignItems: 'center' }}>
        <Pressable
          onPress={goBack}
          accessibilityRole="button"
          accessibilityLabel="Back"
          hitSlop={{ right: 16 }}
          style={{ height: minTouchTarget, flexDirection: 'row', alignItems: 'center' }}
        >
          <Ionicons name="chevron-back" size={24} color={color.ink} />
        </Pressable>
      </View>
      <View style={{ paddingHorizontal: space.lg, marginTop: space.lg }}>{children}</View>
    </SafeAreaView>
  );
}

/**
 * Someone's profile page, opened by a tap on their photo or name in this tab, or by an invite
 * link (`username` in place of `id`, see app/+native-intent.tsx). Your own link opens your Profile.
 * A username that leads to no one (changed, mistyped, gone, or a block either way: the server says
 * not found for all of them alike) shows the gone line of a profile. A lookup that fails shows the
 * plain line with Try again, which looks the username up again. The profile page is only ever handed
 * an id the server gave.
 */
export default function BoardPersonRoute() {
  const router = useRouter();
  const { user } = useSession();
  const params = useLocalSearchParams<{ id?: string | string[]; username?: string | string[] }>();
  const given = Array.isArray(params.id) ? params.id[0] : params.id;
  const username = Array.isArray(params.username) ? params.username[0] : params.username;
  const own = (user?.username ?? '').toLowerCase();
  const [lookup, setLookup] = useState<Lookup>({ state: 'looking' });
  // A new number looks the username up again (Try again).
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    if (given || !username) return;
    if (username === own) {
      router.replace('/(app)/profile');
      return;
    }
    let alive = true;
    lookupPerson(username)
      .then((found) => {
        if (alive) setLookup(found ? { state: 'found', id: found.person.id } : { state: 'gone' });
      })
      .catch((err: unknown) => {
        if (!alive) return;
        const failure = err instanceof ApiError && err.code === 'network_unavailable' ? 'network' : 'generic';
        setLookup({ state: 'failed', failure, retrying: false });
      });
    return () => {
      alive = false;
    };
  }, [given, username, own, router, attempt]);

  function retry() {
    setLookup((now) => (now.state === 'failed' ? { ...now, retrying: true } : now));
    setAttempt((n) => n + 1);
  }

  if (given) return <PersonScreen id={given} />;
  if (lookup.state === 'found') return <PersonScreen id={lookup.id} />;
  if (!username || lookup.state === 'gone') {
    return (
      <LinkPage>
        <AppText role="body">{GONE_LINE}</AppText>
      </LinkPage>
    );
  }
  if (lookup.state === 'failed') {
    return (
      <LinkPage>
        <StatusMessage kind={lookup.failure} onRetry={retry} busy={lookup.retrying} />
      </LinkPage>
    );
  }
  return <View style={{ flex: 1, backgroundColor: color.dominant }} />;
}

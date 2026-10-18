import { useCallback, useState } from 'react';
import { Pressable, ScrollView, View } from 'react-native';
import { useFocusEffect, useRouter } from 'expo-router';
import { Ionicons } from '@expo/vector-icons';
import { SafeAreaView } from 'react-native-safe-area-context';
import { ListTabs, PersonRow, type PeopleList } from '@/components/people/PeopleScreen';
import { StatusMessage } from '@/components/today/StatusMessage';
import { AppText } from '@/components/ui/AppText';
import { ApiError } from '@/lib/api/client';
import { getPeopleOf } from '@/lib/api/person';
import type { Person } from '@/lib/api/social';
import { useOpenPerson } from '@/lib/people/openPerson';
import { color, minTouchTarget, space } from '@/lib/theme/tokens';

/**
 * Another person's Followers and Following, opened from those figures on their profile page, like
 * Instagram: the two lists as tabs under their username. Only an accepted follower gets here (the
 * figures are buttons only then); a row opens that person's profile. Anyone with a block either
 * way with you is left out by the server.
 */
export function PersonListScreen({ id, username, show }: { id: string; username: string; show: PeopleList }) {
  const router = useRouter();
  const openPerson = useOpenPerson();
  const [tab, setTab] = useState<PeopleList>(show);
  const [lists, setLists] = useState<Partial<Record<PeopleList, Person[] | null>>>({});
  const [failure, setFailure] = useState<'network' | 'generic' | null>(null);

  const load = useCallback(async () => {
    try {
      const [followers, following] = await Promise.all([getPeopleOf(id, 'followers'), getPeopleOf(id, 'following')]);
      setLists({ followers, following });
      setFailure(null);
    } catch (err) {
      setFailure(err instanceof ApiError && err.code === 'network_unavailable' ? 'network' : 'generic');
    }
  }, [id]);

  useFocusEffect(
    useCallback(() => {
      void load();
    }, [load]),
  );

  function goBack() {
    if (router.canGoBack()) router.back();
    else router.replace('/(app)/board');
  }

  const people = lists[tab];

  return (
    <SafeAreaView edges={['top', 'left', 'right']} style={{ flex: 1, backgroundColor: color.dominant }}>
      <View style={{ height: minTouchTarget, paddingHorizontal: space.lg, justifyContent: 'center' }}>
        <Pressable
          onPress={goBack}
          accessibilityRole="button"
          accessibilityLabel={`Back to ${username}`}
          hitSlop={{ right: 16 }}
          style={{ height: minTouchTarget, flexDirection: 'row', alignItems: 'center', alignSelf: 'flex-start' }}
        >
          <Ionicons name="chevron-back" size={24} color={color.ink} />
          <View style={{ marginLeft: space.xs }}>
            <AppText role="button">{username}</AppText>
          </View>
        </Pressable>
      </View>

      <View style={{ paddingHorizontal: space.lg, marginTop: space.sm }}>
        <ListTabs value={tab} onChange={setTab} />
      </View>

      <ScrollView contentContainerStyle={{ paddingHorizontal: space.lg, paddingTop: space.md, paddingBottom: space.xl }}>
        {failure !== null && people === undefined ? (
          <StatusMessage kind={failure} onRetry={() => void load()} />
        ) : people === undefined ? (
          <AppText role="label" tone="muted">
            Loading.
          </AppText>
        ) : people === null ? (
          <AppText role="label" tone="muted">
            This list isn't available.
          </AppText>
        ) : people.length === 0 ? (
          <AppText role="label" tone="muted">
            {tab === 'followers' ? 'No followers yet.' : "They don't follow anyone yet."}
          </AppText>
        ) : (
          people.map((person, index) => (
            <PersonRow
              key={person.id}
              person={person}
              first={index === 0}
              onPress={() => openPerson(person.id)}
              pressHint="Opens their profile."
            />
          ))
        )}
      </ScrollView>
    </SafeAreaView>
  );
}

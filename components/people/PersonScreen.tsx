import { useCallback, useRef, useState } from 'react';
import {
  Alert,
  Pressable,
  ScrollView,
  View,
  useWindowDimensions,
  type NativeScrollEvent,
  type NativeSyntheticEvent,
} from 'react-native';
import { useFocusEffect, useRouter, useSegments } from 'expo-router';
import { Ionicons } from '@expo/vector-icons';
import { SafeAreaView } from 'react-native-safe-area-context';
import { PostGrid } from '@/components/profile/PostGrid';
import { SizedAvatar } from '@/components/profile/SizedAvatar';
import { StatsRow } from '@/components/profile/StatsRow';
import { StatusMessage } from '@/components/today/StatusMessage';
import { AppText } from '@/components/ui/AppText';
import { Chip } from '@/components/ui/Chip';
import { MenuSheet, type MenuItem } from '@/components/ui/MenuSheet';
import { blockUser } from '@/lib/api/account';
import { ApiError } from '@/lib/api/client';
import { getPerson, type PersonPage } from '@/lib/api/person';
import { firstName, follow, unfollow } from '@/lib/api/social';
import { postPathIn } from '@/lib/people/openPerson';
import { color, minTouchTarget, space } from '@/lib/theme/tokens';

const AVATAR_SIZE = 72;
const GENERIC = 'Something went wrong. Please try again.';

/**
 * Another person's page, like Instagram's (developer, 2026-10-17: tap anyone's photo or name and
 * it opens their profile). It looks like your own Profile seen by a visitor: their photo, name,
 * @username and the four figures, with one button where your Edit is: Follow, Follow back,
 * Requested (a tap cancels) or Following (a tap offers Unfollow). Block sits in the three dots at
 * the top right. Every follow is a request, so until they accept, the page ends with "This
 * account is private."; an accepted follower sees their grid, only the days they posted too (the
 * Board rule). A tile opens the post page in the same stack. Someone who is gone, or a block
 * either way, shows one plain line.
 */
export function PersonScreen({ id }: { id: string }) {
  const router = useRouter();
  const segments = useSegments() as string[];
  const { width } = useWindowDimensions();
  const [page, setPage] = useState<PersonPage | null>(null);
  const [status, setStatus] = useState<'loading' | 'ready' | 'gone' | 'offline'>('loading');
  const [failure, setFailure] = useState<'network' | 'generic'>('network');
  const [message, setMessage] = useState('');
  const [menu, setMenu] = useState<'more' | 'following' | null>(null);
  const [busy, setBusy] = useState(false);
  const busyRef = useRef(false);
  const loadedRef = useRef(false);
  const pagingRef = useRef(false);

  const load = useCallback(async () => {
    try {
      const next = await getPerson(id);
      if (next === null) {
        setStatus('gone');
        return;
      }
      setPage(next);
      loadedRef.current = true;
      setStatus('ready');
    } catch (err) {
      if (loadedRef.current) return;
      setFailure(err instanceof ApiError && err.code === 'network_unavailable' ? 'network' : 'generic');
      setStatus('offline');
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

  /** The next page of the grid, when the bottom comes within one screen of view. */
  async function loadMore() {
    if (pagingRef.current || page === null || page.posts === null || page.next_cursor === null) return;
    pagingRef.current = true;
    try {
      const more = await getPerson(id, page.next_cursor);
      if (more !== null && more.posts !== null) {
        setPage((now) =>
          now === null ? now : { ...now, posts: [...(now.posts ?? []), ...(more.posts ?? [])], next_cursor: more.next_cursor },
        );
      }
    } catch {
      // The next scroll tries again.
    } finally {
      pagingRef.current = false;
    }
  }

  function handleScroll(event: NativeSyntheticEvent<NativeScrollEvent>) {
    const { contentOffset, contentSize, layoutMeasurement } = event.nativeEvent;
    if (contentOffset.y + layoutMeasurement.height * 2 >= contentSize.height) void loadMore();
  }

  /** Runs one action at a time, then reads the page again; a failure shows one plain line. */
  async function act(work: () => Promise<void>) {
    if (busyRef.current) return;
    busyRef.current = true;
    setBusy(true);
    setMessage('');
    try {
      await work();
      await load();
    } catch (err) {
      setMessage(err instanceof ApiError && err.code === 'network_unavailable' ? err.userMessage : GENERIC);
    } finally {
      busyRef.current = false;
      setBusy(false);
    }
  }

  /** Runs after the menu has closed, so an alert never opens under its dim cover. */
  function afterClose(action: () => void) {
    return () => {
      setMenu(null);
      setTimeout(action, 0);
    };
  }

  function askBlock(name: string) {
    Alert.alert(
      `Block ${name}?`,
      "They won't be able to find you or see your posts, and you won't see theirs. They won't be told.",
      [
        { text: 'Cancel', style: 'cancel' },
        {
          text: 'Block',
          onPress: () =>
            void (async () => {
              try {
                await blockUser(id);
                goBack();
              } catch (err) {
                setMessage(err instanceof ApiError && err.code === 'network_unavailable' ? err.userMessage : GENERIC);
              }
            })(),
        },
      ],
      { cancelable: true },
    );
  }

  const name = page ? firstName(page.person) : '';
  const menuItems: MenuItem[] =
    menu === 'more'
      ? [{ label: `Block ${name}`, onPress: afterClose(() => askBlock(name)) }]
      : menu === 'following'
        ? [{ label: 'Unfollow', onPress: afterClose(() => void act(async () => void (await unfollow(id)))) }]
        : [];

  function relationButton(p: PersonPage) {
    if (p.relation === 'following') {
      return (
        <Chip
          label="Following"
          tone="outline"
          onPress={busy ? undefined : () => setMenu('following')}
          accessibilityLabel={`Following ${name}`}
          accessibilityHint="Opens Unfollow."
        />
      );
    }
    if (p.relation === 'requested') {
      return (
        <Chip
          label="Requested"
          tone="outline"
          onPress={busy ? undefined : () => void act(async () => void (await unfollow(id)))}
          accessibilityLabel={`Requested ${name}`}
          accessibilityHint="Cancels your request."
        />
      );
    }
    const label = p.follows_you ? 'Follow back' : 'Follow';
    return (
      <Chip
        label={label}
        tone="strong"
        onPress={busy ? undefined : () => void act(async () => void (await follow(id)))}
        accessibilityLabel={`${label} ${name}`}
      />
    );
  }

  function openPost(entryId: string) {
    router.push({ pathname: postPathIn(segments), params: { id: entryId, back: page?.person.username ?? '' } });
  }

  return (
    <SafeAreaView edges={['top', 'left', 'right']} style={{ flex: 1, backgroundColor: color.dominant }}>
      {/* The top row: Back with their username, and the three dots. */}
      <View
        style={{
          height: minTouchTarget,
          paddingHorizontal: space.lg,
          flexDirection: 'row',
          alignItems: 'center',
        }}
      >
        <Pressable
          onPress={goBack}
          accessibilityRole="button"
          accessibilityLabel="Back"
          hitSlop={{ right: 16 }}
          style={{ height: minTouchTarget, flex: 1, flexDirection: 'row', alignItems: 'center' }}
        >
          <Ionicons name="chevron-back" size={24} color={color.ink} />
          <View style={{ marginLeft: space.xs, flex: 1 }}>
            <AppText role="button" numberOfLines={1}>
              {page?.person.username ?? ''}
            </AppText>
          </View>
        </Pressable>
        {page !== null ? (
          <Pressable
            onPress={() => setMenu('more')}
            accessibilityRole="button"
            accessibilityLabel={`More for ${name}`}
            hitSlop={8}
            style={{ width: minTouchTarget, height: minTouchTarget, alignItems: 'flex-end', justifyContent: 'center' }}
          >
            <Ionicons name="ellipsis-horizontal" size={24} color={color.ink} />
          </Pressable>
        ) : null}
      </View>

      {status === 'loading' ? (
        <View style={{ paddingHorizontal: space.lg, marginTop: space.lg }}>
          <AppText role="label" tone="muted">
            Loading their profile.
          </AppText>
        </View>
      ) : status === 'offline' ? (
        <View style={{ paddingHorizontal: space.lg, marginTop: space.lg }}>
          <StatusMessage kind={failure} onRetry={() => void load()} />
        </View>
      ) : status === 'gone' || page === null ? (
        <View style={{ paddingHorizontal: space.lg, marginTop: space.lg }}>
          <AppText role="body">This account isn't available.</AppText>
        </View>
      ) : (
        <ScrollView onScroll={handleScroll} scrollEventThrottle={200} contentContainerStyle={{ paddingBottom: space.xl }}>
          <View style={{ paddingHorizontal: space.lg, paddingTop: space.sm }}>
            <View style={{ flexDirection: 'row', alignItems: 'center' }}>
              <SizedAvatar
                size={AVATAR_SIZE}
                name={page.person.name || page.person.username}
                uri={page.person.photo_url}
                placeholder="initials"
              />
              <View style={{ flex: 1, marginLeft: space.md }}>
                {page.person.name !== '' ? (
                  <AppText role="note" numberOfLines={2} ellipsizeMode="tail" accessibilityRole="header">
                    {page.person.name}
                  </AppText>
                ) : null}
                <View style={{ marginTop: page.person.name !== '' ? space.xs : 0 }}>
                  <AppText role="body" tone="muted" numberOfLines={1} ellipsizeMode="tail">
                    {`@${page.person.username}`}
                  </AppText>
                </View>
              </View>
              <View style={{ marginLeft: space.md }}>{relationButton(page)}</View>
            </View>

            {message !== '' ? (
              <View style={{ marginTop: space.sm }} accessibilityLiveRegion="polite">
                <AppText role="label">{message}</AppText>
              </View>
            ) : null}

            <View style={{ marginTop: space.lg }}>
              <StatsRow
                spins={page.stats.spins}
                weekStreak={page.stats.week_streak}
                followers={page.stats.followers}
                following={page.stats.following}
              />
            </View>
          </View>

          {page.posts === null ? (
            <View style={{ paddingHorizontal: space.lg, marginTop: space.xl, alignItems: 'center' }}>
              <Ionicons name="lock-closed-outline" size={28} color={color.ink} />
              <View style={{ marginTop: space.sm }}>
                <AppText role="button">This account is private.</AppText>
              </View>
              <View style={{ marginTop: space.xs }}>
                <AppText role="label" tone="muted">
                  {page.relation === 'requested'
                    ? `You'll see their posts once ${name} accepts.`
                    : 'Follow to see their posts.'}
                </AppText>
              </View>
            </View>
          ) : (
            <View style={{ marginTop: space.lg }}>
              <PostGrid
                posts={page.posts}
                width={width}
                onOpen={openPost}
                emptyText="Their posts show here on days you post too."
              />
            </View>
          )}
        </ScrollView>
      )}
      <MenuSheet visible={menu !== null} items={menuItems} onClose={() => setMenu(null)} />
    </SafeAreaView>
  );
}

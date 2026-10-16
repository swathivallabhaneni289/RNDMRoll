import { useRef, useState } from 'react';
import { Pressable, ScrollView, Text, View, type NativeScrollEvent, type NativeSyntheticEvent } from 'react-native';
import { useRouter } from 'expo-router';
import { Ionicons } from '@expo/vector-icons';
import { SafeAreaView } from 'react-native-safe-area-context';
import { useDiary } from '@/components/diary/useDiary';
import { Details, Gallery } from '@/components/entry/EntryView';
import { SizedAvatar } from '@/components/profile/SizedAvatar';
import { useEntryMenu } from '@/components/entry/useEntryMenu';
import { StatusMessage } from '@/components/today/StatusMessage';
import { AppText } from '@/components/ui/AppText';
import { MenuSheet } from '@/components/ui/MenuSheet';
import { TextButton } from '@/components/ui/TextButton';
import type { Entry } from '@/lib/api/entries';
import { useReduceMotion } from '@/lib/motion/useReduceMotion';
import { useSession } from '@/lib/session/store';
import { color, minTouchTarget, space, type } from '@/lib/theme/tokens';

/** Your small round photo at the left of each post's top row. */
const AVATAR = 32;

/** The photos in the list are 4 wide to 5 high, like Instagram's posts. */
const PHOTO_ASPECT = 4 / 5;

/**
 * Your posts as one scrolling list, opened from a picture in the Profile grid and placed at that
 * picture (developer, 2026-10-17: "how Instagram lets you see the pictures you posted"). Each post
 * has a top row with your photo, your username and its three dots (Edit, Archive, commenting, Delete), the
 * photos to swipe, the title, stars and note, and "View comments", which opens the full entry
 * page with the reactions and comments. The list pages as the end comes into view.
 */
export function PostsFeed({ startId }: { startId: string }) {
  const router = useRouter();
  const reduceMotion = useReduceMotion();
  const { user } = useSession();
  const name = user?.name || user?.username || '';
  const handle = user?.username ?? '';
  const diary = useDiary(null);
  const posts = diary.rows.flatMap((row) => (row.kind === 'entry' ? [row.entry] : []));
  const menu = useEntryMenu('profile', diary.reload);

  const scrollRef = useRef<ScrollView>(null);
  // Jump to the tapped post once, as soon as it has been laid out.
  const [placed, setPlaced] = useState(false);

  function goBack() {
    if (router.canGoBack()) router.back();
    else router.replace('/(app)/profile');
  }

  function handleScroll(event: NativeSyntheticEvent<NativeScrollEvent>) {
    const { contentOffset, contentSize, layoutMeasurement } = event.nativeEvent;
    if (contentOffset.y + layoutMeasurement.height * 2 >= contentSize.height) diary.loadMore();
  }

  function renderPost(entry: Entry) {
    return (
      <View
        key={entry.id}
        onLayout={(event) => {
          if (placed || entry.id !== startId) return;
          scrollRef.current?.scrollTo({ y: event.nativeEvent.layout.y, animated: false });
          setPlaced(true);
        }}
        style={{ marginBottom: space.xl }}
      >
        <View
          style={{
            height: minTouchTarget,
            paddingHorizontal: space.lg,
            flexDirection: 'row',
            alignItems: 'center',
          }}
        >
          <View
            accessibilityElementsHidden
            importantForAccessibility="no-hide-descendants"
            style={{ flex: 1, flexDirection: 'row', alignItems: 'center' }}
          >
            <SizedAvatar size={AVATAR} name={name} uri={user?.avatar_url ?? null} placeholder="initials" />
            <View style={{ marginLeft: space.sm, flex: 1 }}>
              <AppText role="button" numberOfLines={1}>
                {handle}
              </AppText>
            </View>
          </View>
          <Pressable
            onPress={() => menu.open(entry)}
            disabled={menu.busy}
            accessibilityRole="button"
            accessibilityLabel={`More for ${entry.title}`}
            hitSlop={8}
            style={{ width: minTouchTarget, height: minTouchTarget, alignItems: 'flex-end', justifyContent: 'center' }}
          >
            <Ionicons name="ellipsis-horizontal" size={24} color={menu.busy ? color.muted : color.ink} />
          </Pressable>
        </View>
        <Gallery photos={entry.photos} title={entry.title} aspect={PHOTO_ASPECT} />
        <Details entry={entry} reduceMotion={reduceMotion} />
        <View style={{ paddingHorizontal: space.lg, alignItems: 'flex-start' }}>
          <TextButton
            label="View comments"
            onPress={() => router.push({ pathname: '/(app)/profile/entry', params: { id: entry.id, from: 'profile' } })}
          />
        </View>
      </View>
    );
  }

  return (
    <SafeAreaView edges={['top']} style={{ flex: 1, backgroundColor: color.dominant }}>
      <View style={{ height: minTouchTarget, paddingHorizontal: space.lg, justifyContent: 'center' }}>
        <Pressable
          onPress={goBack}
          accessibilityRole="button"
          accessibilityLabel="Back to Profile"
          hitSlop={{ right: 16 }}
          style={{ height: minTouchTarget, flexDirection: 'row', alignItems: 'center', alignSelf: 'flex-start' }}
        >
          <Ionicons name="chevron-back" size={24} color={color.ink} />
          <View style={{ marginLeft: space.xs }}>
            <AppText role="body">Posts</AppText>
          </View>
        </Pressable>
      </View>

      {menu.message !== null ? (
        <View style={{ paddingHorizontal: space.lg, paddingVertical: space.sm }} accessibilityLiveRegion="polite">
          <Text
            style={{
              fontFamily: type.label.fontFamily,
              fontSize: type.label.fontSize,
              lineHeight: type.label.lineHeight,
              color: color.ink,
              textAlign: 'center',
            }}
          >
            {menu.message}
          </Text>
        </View>
      ) : null}

      {diary.status === 'loading' ? (
        <View style={{ paddingHorizontal: space.lg, marginTop: space.xl }}>
          <AppText role="label" tone="muted">
            Loading your posts.
          </AppText>
        </View>
      ) : diary.status === 'offline' ? (
        <View style={{ paddingHorizontal: space.lg, marginTop: space.xl }}>
          <StatusMessage kind={diary.failure} onRetry={diary.retry} />
        </View>
      ) : (
        <ScrollView
          ref={scrollRef}
          onScroll={handleScroll}
          scrollEventThrottle={200}
          showsVerticalScrollIndicator={false}
        >
          {posts.map(renderPost)}
        </ScrollView>
      )}
      <MenuSheet visible={menu.visible} items={menu.items} onClose={menu.close} />
    </SafeAreaView>
  );
}

import { useEffect, useRef, useState } from 'react';
import { FlatList, View } from 'react-native';
import { useLocalSearchParams } from 'expo-router';
import { useReducedMotion } from 'react-native-reanimated';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { FadeInUp, staggerDelay } from '@/lib/motion/primitives';
import { color, space } from '@/lib/theme/tokens';
import { FEED, formatLong, LabEntry } from '@/lab/data/entries';
import { HeroClipContext } from '@/lab/motion/hero';
import { FeedPost } from '@/lab/components/FeedPost';
import { LabBackLink } from '@/lab/components/LabBackLink';
import { LabText } from '@/lab/components/LabText';

/** Reveal only the first screenful, once per visit; later rows just scroll in. */
const REVEALED_ROWS = 3;

function FeedRow({
  entry,
  index,
  seen,
  autoOpen,
}: {
  entry: LabEntry;
  index: number;
  seen: Set<string>;
  autoOpen: boolean;
}) {
  const reduced = useReducedMotion();
  // Decided once at mount and never flipped, so the row keeps the same element tree.
  const [animate] = useState(() => !reduced && index < REVEALED_ROWS && !seen.has(entry.id));
  useEffect(() => {
    seen.add(entry.id);
  }, [seen, entry.id]);

  const post = <FeedPost entry={entry} autoOpen={autoOpen} />;
  return animate ? (
    <FadeInUp delay={staggerDelay(index, 60, 90)} duration={420}>
      {post}
    </FadeInUp>
  ) : (
    post
  );
}

function Header() {
  return (
    <View style={{ paddingBottom: space.xl }}>
      <LabBackLink />
      <LabText role="display" style={{ marginTop: space.lg }}>
        Friends
      </LabText>
      <LabText role="label" tone="muted" style={{ marginTop: space.xs }}>
        {formatLong(new Date())}
      </LabText>
    </View>
  );
}

/** Posts have no card or border, so generous whitespace is what tells them apart. */
function Separator() {
  return <View style={{ height: space.xxl }} />;
}

/**
 * Screen A: today's board. Warm off-white ground, one post per entry: header, the
 * photograph, then the caption and the latest comments below it. Nothing sits on a
 * photograph. The first few posts reveal in sequence on first load.
 */
export default function FeedScreen() {
  const insets = useSafeAreaInsets();
  const seen = useRef(new Set<string>()).current;
  const list = useRef<FlatList<LabEntry>>(null);
  // Dev aids for recording on a simulator without touch automation:
  //   /lab/feed?open=f0   open that entry by itself after a moment
  //   /lab/feed?y=1080    start scrolled to that offset
  const { open, y } = useLocalSearchParams<{ open?: string; y?: string }>();

  useEffect(() => {
    const offset = Number(y);
    if (!Number.isFinite(offset) || offset <= 0) return;
    const timer = setTimeout(() => list.current?.scrollToOffset({ offset, animated: false }), 500);
    return () => clearTimeout(timer);
    // Mount-only dev aid.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <HeroClipContext.Provider value={insets.top}>
    <View style={{ flex: 1, backgroundColor: color.dominant }}>
      <FlatList
        ref={list}
        data={FEED}
        keyExtractor={(entry) => entry.id}
        renderItem={({ item, index }) => (
          <FeedRow entry={item} index={index} seen={seen} autoOpen={open === item.id} />
        )}
        ListHeaderComponent={Header}
        ItemSeparatorComponent={Separator}
        initialNumToRender={3}
        maxToRenderPerBatch={2}
        windowSize={5}
        showsVerticalScrollIndicator={false}
        contentContainerStyle={{
          paddingTop: insets.top + space.sm,
          paddingBottom: insets.bottom + space.xxl,
          paddingHorizontal: space.lg,
        }}
      />
      {/* Solid cover so scrolled photographs and text never collide with the clock. */}
      <View
        style={{
          position: 'absolute',
          top: 0,
          left: 0,
          right: 0,
          height: insets.top,
          backgroundColor: color.dominant,
          pointerEvents: 'none',
        }}
      />
    </View>
    </HeroClipContext.Provider>
  );
}

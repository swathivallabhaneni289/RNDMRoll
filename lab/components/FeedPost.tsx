import { memo, useEffect } from 'react';
import { Pressable, useWindowDimensions, View } from 'react-native';
import { useRouter } from 'expo-router';
import { PressScale } from '@/lib/motion/primitives';
import { color, minTouchTarget, space, type } from '@/lib/theme/tokens';
import { categoryLabel, formatTime, LabEntry } from '@/lab/data/entries';
import { usePhoto } from '@/lab/data/photos';
import { useHeroSource } from '@/lab/motion/hero';
import { photoMetrics } from '@/lab/theme/frost';
import { Avatar, AVATAR_GAP } from './Avatar';
import { LabPhoto } from './LabPhoto';
import { LabText } from './LabText';
import { StarRating } from './StarRating';

/** As tall as a touch target, so the header lines up with the detail's close row. */
const HEADER_MIN_HEIGHT = minTouchTarget;
/** The feed shows the latest few comments; the rest are one tap away in the detail. */
const VISIBLE_COMMENTS = 2;
/** A label line is shorter than a touch target, so the link reaches one with slop above and below. */
const LINK_SLOP = (minTouchTarget - type.label.lineHeight) / 2;
const LINK_HIT_SLOP = { top: LINK_SLOP, bottom: LINK_SLOP };

/**
 * One friend's entry on today's board, laid out top to bottom like a post: who and
 * when, the photograph, then the caption and the latest comments on the plain ground.
 * Nothing sits on the photo. There is no card, border or shadow; whitespace between
 * posts (set by the list) is the only divider.
 *
 * The shared-element hook is attached to the photo container alone, so the detail
 * flies out of the photograph and never out of the whole post. The photo, "View all"
 * and "Add a comment" all open the same detail through one handler.
 */
export const FeedPost = memo(function FeedPost({ entry, autoOpen = false }: { entry: LabEntry; autoOpen?: boolean }) {
  const router = useRouter();
  const { width } = useWindowDimensions();
  const photo = usePhoto(entry);
  const photoWidth = width - space.lg * 2;
  const photoHeight = Math.round(photoWidth / photoMetrics.aspect);
  const { ref, open, hidden } = useHeroSource(`feed:${entry.id}`, photoMetrics.feedRadius);

  async function handlePress() {
    if (await open()) {
      router.push({ pathname: '/lab/entry/[id]', params: { id: entry.id, from: 'feed' } });
    }
  }

  // Dev aid: /lab/feed?open=<id> opens this entry by itself, so the transition can be
  // recorded on a simulator without touch automation.
  useEffect(() => {
    if (!autoOpen) return;
    const timer = setTimeout(() => {
      void handlePress();
    }, 1600);
    return () => clearTimeout(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [autoOpen]);

  const total = entry.comments.length;
  const latest = entry.comments.slice(-VISIBLE_COMMENTS);
  const hasMore = total > VISIBLE_COMMENTS;

  return (
    <View testID={`feed-post-${entry.id}`}>
      <View
        style={{
          minHeight: HEADER_MIN_HEIGHT,
          marginBottom: space.sm,
          flexDirection: 'row',
          alignItems: 'center',
          gap: AVATAR_GAP,
        }}
      >
        <Avatar name={entry.author} />
        <View style={{ flex: 1 }}>
          <LabText role="button" numberOfLines={1}>
            {entry.author}
          </LabText>
          <LabText role="label" tone="muted" numberOfLines={1}>
            {`${categoryLabel(entry.category)} · ${formatTime(entry.date)}`}
          </LabText>
        </View>
      </View>

      <PressScale
        onPress={handlePress}
        scaleTo={0.985}
        accessibilityRole="button"
        accessibilityLabel={`Photo of ${entry.title} by ${entry.author}`}
        accessibilityHint="Opens the entry"
        testID={`feed-item-${entry.id}`}
      >
        <View
          ref={ref}
          collapsable={false}
          style={{
            width: photoWidth,
            height: photoHeight,
            borderRadius: photoMetrics.feedRadius,
            overflow: 'hidden',
            backgroundColor: color.secondary,
            opacity: hidden ? 0 : 1,
          }}
        >
          <LabPhoto photo={photo} />
        </View>
      </PressScale>

      <View style={{ marginTop: space.md }}>
        <LabText role="heading" numberOfLines={2}>
          {entry.title}
        </LabText>
        <View style={{ marginTop: space.xs }}>
          <StarRating value={entry.rating} size={14} />
        </View>
        <LabText role="body" style={{ marginTop: space.sm }}>
          <LabText role="body" strong>
            {entry.author}
          </LabText>
          {` ${entry.reaction}`}
        </LabText>
      </View>

      <View style={{ marginTop: space.sm }}>
        {hasMore ? (
          <Pressable
            onPress={handlePress}
            hitSlop={LINK_HIT_SLOP}
            accessibilityRole="button"
            accessibilityHint="Opens the entry"
            style={({ pressed }) => ({ opacity: pressed ? 0.6 : 1 })}
          >
            <LabText role="label" tone="muted">
              {`View all ${total} comments`}
            </LabText>
          </Pressable>
        ) : null}
        {total === 0 ? (
          <Pressable
            onPress={handlePress}
            hitSlop={LINK_HIT_SLOP}
            accessibilityRole="button"
            accessibilityHint="Opens the entry"
            style={({ pressed }) => ({ opacity: pressed ? 0.6 : 1 })}
          >
            <LabText role="label" tone="muted">
              Add a comment
            </LabText>
          </Pressable>
        ) : null}
        {latest.map((comment, index) => (
          <LabText key={comment.id} role="label" style={{ marginTop: hasMore || index > 0 ? space.xs : 0 }}>
            <LabText role="label" strong>
              {comment.author}
            </LabText>
            {` ${comment.text}`}
          </LabText>
        ))}
      </View>
    </View>
  );
});

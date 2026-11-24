import { useEffect, useState } from 'react';
import { Pressable, Text, View } from 'react-native';
import Animated, { useAnimatedStyle, useSharedValue, withSequence, withSpring, withTiming } from 'react-native-reanimated';
import { Ionicons } from '@expo/vector-icons';
import { StarDisplay } from '@/components/entry/StarDisplay';
import type { Entry } from '@/lib/api/entries';
import { LIKE_LABEL, LIKES_HINT, likeCount, likesSpoken } from '@/lib/board/text';
import { REACTIONS } from '@/lib/entry/reactions';
import { starsSpoken } from '@/lib/entry/stars';
import { EASE_OUT } from '@/lib/motion/primitives';
import { useReduceMotion } from '@/lib/motion/useReduceMotion';
import { color, minTouchTarget, space, type } from '@/lib/theme/tokens';
import { dateLineLong } from '@/lib/wheel/format';

/**
 * The words under a post, shared by the Profile posts list, your entry page and someone else's
 * post page so every post reads the same (developer, 2026-10-18): the ActionRow right under the
 * photo, then CaptionText. One font, all ink, bold only for names, the category and the title.
 */

/** What the caption needs: an Entry has it, and so does a Board post (its card and note). */
export type CaptionPost = Pick<Entry, 'title' | 'stars' | 'reaction' | 'comment' | 'local_date' | 'late'> & {
  category: { name: string };
};

/** Your posts' photos are 4 wide to 5 high, like Instagram's, in the list and on the entry page. */
export const POST_PHOTO_ASPECT = 4 / 5;

const STAR_SIZE = 18;
const OWN_REACTION = 20;
const COMMENT_ICON = 24;

/** Every line under a post: the app's body size in ink, so nothing reads bigger or greyer. */
export const captionLine = {
  fontFamily: type.body.fontFamily,
  fontSize: type.body.fontSize,
  lineHeight: type.body.lineHeight,
  color: color.ink,
} as const;
export const captionBold = { fontFamily: type.button.fontFamily } as const;
/** The category and day: the same font and ink, one step smaller. */
const small = {
  fontFamily: type.label.fontFamily,
  fontSize: type.label.fontSize,
  lineHeight: type.label.lineHeight,
  color: color.ink,
} as const;

/** "SUNDAY, OCTOBER 18" as "Sunday, October 18": the caption reads in plain words, not capitals. */
function plainDate(localDate: string): string {
  return dateLineLong(localDate)
    .toLowerCase()
    .replace(/\b[a-z]/g, (letter) => letter.toUpperCase());
}

/**
 * The heart under a post: on someone else's post a toggle (`liked`, and a tap likes it or undoes
 * it), on your own the count, whose tap opens the Likes list.
 */
export type LikeControl =
  | { mode: 'toggle'; liked: boolean; onToggle: () => void }
  | { mode: 'count'; count: number; onOpen: () => void };

/**
 * The heart, 24 like the comment mark and ink whether it is filled or not (no red, no emoji). A
 * toggle pops on each tap, the same grow and settle as an emoji being picked (ReactionPicker), and
 * never on the first draw, under Reduce Motion, or when a like that would not save puts the heart
 * back. The count has no pop, no number at none, and stays a button at none.
 */
function LikeButton({ like }: { like: LikeControl }) {
  const reduceMotion = useReduceMotion();
  const scale = useSharedValue(1);
  const popStyle = useAnimatedStyle(() => ({ transform: [{ scale: scale.value }] }));
  // Each tap counts here, so only a tap pops the heart.
  const [taps, setTaps] = useState(0);
  useEffect(() => {
    if (taps === 0 || reduceMotion) return;
    scale.value = withSequence(
      withTiming(1.25, { duration: 120, easing: EASE_OUT }),
      withSpring(1, { damping: 12, stiffness: 260 }),
    );
    // A tap alone starts it: Reduce Motion turning off afterwards never does.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [taps]);

  if (like.mode === 'count') {
    const number = likeCount(like.count);
    return (
      <Pressable
        onPress={like.onOpen}
        accessibilityRole="button"
        accessibilityLabel={likesSpoken(like.count)}
        accessibilityHint={LIKES_HINT}
        hitSlop={8}
        style={{ height: minTouchTarget, flexDirection: 'row', alignItems: 'center' }}
      >
        <Ionicons name="heart-outline" size={COMMENT_ICON} color={color.ink} />
        {number !== '' ? (
          <Text maxFontSizeMultiplier={1.3} style={[captionLine, { marginLeft: space.xs }]}>
            {number}
          </Text>
        ) : null}
      </Pressable>
    );
  }
  const { liked, onToggle } = like;
  return (
    <Pressable
      onPress={() => {
        setTaps((n) => n + 1);
        onToggle();
      }}
      accessibilityRole="button"
      accessibilityLabel={LIKE_LABEL}
      accessibilityState={{ selected: liked }}
      hitSlop={8}
      style={{ height: minTouchTarget, justifyContent: 'center' }}
    >
      <Animated.View style={popStyle}>
        <Ionicons name={liked ? 'heart' : 'heart-outline'} size={COMMENT_ICON} color={color.ink} />
      </Animated.View>
    </Pressable>
  );
}

/**
 * The row right under the photo, where Instagram keeps its icons: your stars and own emoji at the
 * left, and at the right the heart, then the comment mark (`onComment`: the list opens the post,
 * the entry page puts the cursor in the comment box). The heart (`like`, developer, 2026-11-24: "if
 * they dont comment atleast people can just like it and move on") likes someone else's post and
 * undoes it, with a short pop (none under Reduce Motion); on your own post it shows how many liked
 * it, with no number at none, and opens the Likes list. It stays when comments are off. The emoji
 * at the left is the author's own pick, not a like.
 */
export function ActionRow({
  entry,
  like,
  onComment,
  whose = 'your',
}: {
  entry: CaptionPost;
  /** The heart; none until the post's likes are known. */
  like?: LikeControl;
  onComment?: () => void;
  /** Whose emoji it is, for a screen reader: yours, or the author's on someone else's post. */
  whose?: 'your' | 'their';
}) {
  const own = entry.reaction === null ? null : (REACTIONS.find((item) => item.name === entry.reaction) ?? null);
  return (
    <View style={{ height: minTouchTarget, paddingHorizontal: space.lg, flexDirection: 'row', alignItems: 'center' }}>
      <View
        accessible
        accessibilityLabel={
          own ? `${starsSpoken(entry.stars)}. ${whose === 'your' ? 'Your' : 'Their'} reaction: ${own.label}.` : `${starsSpoken(entry.stars)}.`
        }
        style={{ flexDirection: 'row', alignItems: 'center' }}
      >
        <StarDisplay value={entry.stars} size={STAR_SIZE} decorative />
        {own ? (
          <Text allowFontScaling={false} style={{ marginLeft: space.sm, fontSize: OWN_REACTION, lineHeight: type.body.lineHeight }}>
            {own.char}
          </Text>
        ) : null}
      </View>
      {like || onComment ? (
        // The heart never waits on the comment mark: with comments off only the mark goes. The gap
        // keeps the two hit areas apart.
        <View style={{ marginLeft: 'auto', flexDirection: 'row', alignItems: 'center' }}>
          {like ? <LikeButton like={like} /> : null}
          {onComment ? (
            <Pressable
              onPress={onComment}
              accessibilityRole="button"
              accessibilityLabel="Comments"
              hitSlop={8}
              style={{ marginLeft: like ? space.md : 0, height: minTouchTarget, justifyContent: 'center' }}
            >
              <Ionicons name="chatbubble-outline" size={COMMENT_ICON} color={color.ink} />
            </Pressable>
          ) : null}
        </View>
      ) : null}
    </View>
  );
}

/**
 * What the post is about and its day ('Song · Sunday, October 18', one step smaller, then the
 * posted time when given, and 'Posted late' when it was), the title in bold over the full width,
 * and the author's note.
 */
export function CaptionText({ entry, time }: { entry: CaptionPost; time?: string }) {
  const note = entry.comment !== null && entry.comment.trim() !== '' ? entry.comment : null;
  const when = [plainDate(entry.local_date), time ?? '', entry.late ? 'Posted late' : ''].filter((part) => part !== '').join(' · ');
  return (
    <View style={{ paddingHorizontal: space.lg }}>
      <Text numberOfLines={1} maxFontSizeMultiplier={1.3} accessibilityLabel={`${entry.category.name}. ${when.replace(/ · /g, '. ')}.`} style={small}>
        <Text style={captionBold}>{entry.category.name}</Text>
        {` · ${when}`}
      </Text>
      <Text maxFontSizeMultiplier={1.3} style={[captionLine, captionBold, { marginTop: space.xs }]}>
        {entry.title}
      </Text>
      {note !== null ? (
        <Text maxFontSizeMultiplier={1.3} style={[captionLine, { marginTop: space.xs }]}>
          {note}
        </Text>
      ) : null}
    </View>
  );
}

import { Pressable, Text, View } from 'react-native';
import { Ionicons } from '@expo/vector-icons';
import { StarDisplay } from '@/components/entry/StarDisplay';
import type { Entry } from '@/lib/api/entries';
import { REACTIONS } from '@/lib/entry/reactions';
import { starsSpoken } from '@/lib/entry/stars';
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
 * The row right under the photo, where Instagram keeps its icons: your stars and own emoji at the
 * left, and the comment mark at the right (`onComment`: the list opens the post, the entry page
 * puts the cursor in the comment box). Only things that work here; there are no likes or
 * reactions on posts (developer, 2026-10-18).
 */
export function ActionRow({
  entry,
  onComment,
  whose = 'your',
}: {
  entry: CaptionPost;
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
      {onComment ? (
        <Pressable
          onPress={onComment}
          accessibilityRole="button"
          accessibilityLabel="Comments"
          hitSlop={8}
          style={{ marginLeft: 'auto', height: minTouchTarget, justifyContent: 'center' }}
        >
          <Ionicons name="chatbubble-outline" size={COMMENT_ICON} color={color.ink} />
        </Pressable>
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

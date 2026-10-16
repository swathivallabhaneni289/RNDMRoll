import { Pressable, Text, View } from 'react-native';
import { Image } from 'expo-image';
import { useRouter } from 'expo-router';
import { StarDisplay } from '@/components/entry/StarDisplay';
import { CapsLabel } from '@/components/today/CapsLabel';
import { AppText } from '@/components/ui/AppText';
import type { Today, TodayEntry } from '@/lib/api/types';
import { REACTIONS } from '@/lib/entry/reactions';
import { settledLabel } from '@/lib/entry/rules';
import { starsSpoken } from '@/lib/entry/stars';
import { ImageReveal } from '@/lib/motion/primitives';
import { useReduceMotion } from '@/lib/motion/useReduceMotion';
import { color, space, type } from '@/lib/theme/tokens';

/** The cover thumbnail of the Posted row (03-UI-SPEC 3.3, picture 04): 4 to 5, portrait. */
const THUMB_WIDTH = 78;
const THUMB_HEIGHT = 98;
/** The stars and the reaction character on the row's last line. */
const STARS_SIZE = 16;
const REACTION_SIZE = 18;
/** The strike through the word in Skipped: 2dp thick, 4dp longer than the word at both ends. */
const STRIKE_THICKNESS = 2;
const STRIKE_OVERHANG = 4;

/**
 * The entry as one row (picture 04): the cover thumbnail with '+N' for the other photos, and at its
 * right the category, the title, the stars and the reaction. Everything comes from `Today.entry`
 * and the shown spin's category name: no second request and no loading state. 'Posted late.' sits
 * under the row, a hairline closes it; an entry that was deleted leaves one line in its place
 * (the day still counts). 'View entry' sits at the bottom of the text column, outside the spoken
 * group so it stays its own button, and opens the entry page with `from` = 'today'.
 */
function PostedRow({ entry, categoryName }: { entry: TodayEntry | null; categoryName: string }) {
  const reduceMotion = useReduceMotion();
  const router = useRouter();

  if (entry === null) {
    return (
      <AppText role="label" tone="muted">
        This entry was deleted. The day still counts.
      </AppText>
    );
  }

  if (entry.archived) {
    return (
      <AppText role="label" tone="muted">
        This entry is archived. The day still counts.
      </AppText>
    );
  }

  const reaction = entry.reaction === null ? null : (REACTIONS.find((item) => item.name === entry.reaction) ?? null);
  const others = entry.photo_count - 1;
  const spoken = [
    categoryName !== '' ? `${categoryName}.` : null,
    `${entry.title}.`,
    `${starsSpoken(entry.stars)}.`,
    reaction ? `Reaction: ${reaction.label}.` : null,
  ]
    .filter((part): part is string => part !== null)
    .join(' ');

  const image =
    entry.photo_url !== '' ? (
      <Image source={{ uri: entry.photo_url }} style={{ width: '100%', height: '100%' }} contentFit="cover" />
    ) : null;

  return (
    <View>
      <View style={{ flexDirection: 'row', alignItems: 'flex-start' }}>
        <View
          accessibilityElementsHidden
          importantForAccessibility="no-hide-descendants"
          style={{ width: THUMB_WIDTH, height: THUMB_HEIGHT, backgroundColor: color.secondary }}
        >
          {image === null ? null : reduceMotion ? image : <ImageReveal style={{ flex: 1 }}>{image}</ImageReveal>}
          {others > 0 ? (
            <View
              style={{
                position: 'absolute',
                right: 0,
                bottom: 0,
                padding: space.xs,
                backgroundColor: color.dominant,
              }}
            >
              <Text
                style={{
                  fontFamily: type.label.fontFamily,
                  fontSize: type.label.fontSize,
                  lineHeight: type.label.lineHeight,
                  color: color.ink,
                }}
              >
                {`+${others}`}
              </Text>
            </View>
          ) : null}
        </View>

        <View style={{ flex: 1, marginLeft: space.md }}>
          <View accessible accessibilityLabel={spoken}>
            <CapsLabel numberOfLines={1}>{categoryName}</CapsLabel>
            <View style={{ marginTop: space.xs }}>
              <AppText role="note" numberOfLines={2}>
                {entry.title}
              </AppText>
            </View>
            <View style={{ marginTop: space.xs, flexDirection: 'row', alignItems: 'center' }}>
              <StarDisplay value={entry.stars} size={STARS_SIZE} decorative />
              {reaction ? (
                <Text
                  allowFontScaling={false}
                  style={{
                    marginLeft: space.sm,
                    fontSize: REACTION_SIZE,
                    lineHeight: type.body.lineHeight,
                  }}
                >
                  {reaction.char}
                </Text>
              ) : null}
            </View>
          </View>
          <View style={{ marginTop: space.sm, alignItems: 'flex-start' }}>
            <Pressable
              onPress={() =>
                router.push({ pathname: '/(app)/today/entry', params: { id: entry.id, from: 'today' } })
              }
              accessibilityRole="button"
              accessibilityLabel="View entry"
              hitSlop={{ top: 10, bottom: 10, left: 8, right: 16 }}
            >
              <Text
                style={{
                  fontFamily: type.body.fontFamily,
                  fontSize: type.body.fontSize,
                  lineHeight: type.body.lineHeight,
                  color: color.ink,
                  textDecorationLine: 'underline',
                }}
              >
                View entry
              </Text>
            </Pressable>
          </View>
        </View>
      </View>

      {entry.late ? (
        <View style={{ marginTop: space.sm }}>
          <AppText role="label" tone="muted">
            Posted late.
          </AppText>
        </View>
      ) : null}

      <View style={{ marginTop: space.md, height: 1, backgroundColor: color.divider }} />
    </View>
  );
}

/**
 * The word of a skipped day (picture 05): the small-caps label, and under it the category in the
 * `heroSmall` role in `inkInactive`, one line that shrinks to fit, with a 2dp `ink` line through
 * its middle that is 4dp longer than the word at both ends. One spoken element.
 */
function SkippedWord({ label, name }: { label: string; name: string }) {
  return (
    <View accessible accessibilityLabel={`${label} ${name}. Skipped.`}>
      <CapsLabel>{label}</CapsLabel>
      <View style={{ marginTop: space.sm, alignSelf: 'flex-start', maxWidth: '100%' }}>
        <Text
          numberOfLines={1}
          adjustsFontSizeToFit
          minimumFontScale={0.36}
          allowFontScaling={false}
          style={{
            fontFamily: type.heroSmall.fontFamily,
            fontSize: type.heroSmall.fontSize,
            lineHeight: type.heroSmall.lineHeight,
            color: color.inkInactive,
          }}
        >
          {name}
        </Text>
        <View
          pointerEvents="none"
          style={{
            position: 'absolute',
            left: -STRIKE_OVERHANG,
            right: -STRIKE_OVERHANG,
            top: (type.heroSmall.lineHeight - STRIKE_THICKNESS) / 2,
            height: STRIKE_THICKNESS,
            backgroundColor: color.ink,
          }}
        />
      </View>
    </View>
  );
}

/**
 * What takes the word block's place once the day is settled (03-UI-SPEC 3.3 and 3.4): the posted
 * entry as a row, or the struck word of a skipped day. Any other state draws nothing. The sentence
 * under the heading in Skipped belongs to the screen, not to this block.
 */
export function SettledBlock({ today }: { today: Today }) {
  if (today.state === 'skipped') {
    const spin = today.spin;
    if (!spin) return null;
    return <SkippedWord label={settledLabel(spin.local_date, today.local_date)} name={spin.category.name} />;
  }
  if (today.state === 'posted') {
    return <PostedRow entry={today.entry ?? null} categoryName={today.spin?.category.name ?? ''} />;
  }
  return null;
}

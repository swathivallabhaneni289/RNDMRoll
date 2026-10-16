import { useState } from 'react';
import { Pressable, Text, View } from 'react-native';
import { Image } from 'expo-image';
import { Ionicons } from '@expo/vector-icons';
import { StarDisplay } from '@/components/entry/StarDisplay';
import { AppText } from '@/components/ui/AppText';
import { datePrefix, quotedNote, rowSpoken, skippedSpoken, starsFigure } from '@/components/diary/diaryText';
import type { Entry } from '@/lib/api/entries';
import { color, minTouchTarget, radius, space, type } from '@/lib/theme/tokens';

const COVER = 76;
const ROW_MIN_HEIGHT = 88;
const SKIPPED_MIN_HEIGHT = 56;

/** The Label role as a plain text style, so a line can be cut in parts (the category first). */
const labelStyle = {
  fontFamily: type.label.fontFamily,
  fontSize: type.label.fontSize,
  lineHeight: type.label.lineHeight,
  color: color.muted,
} as const;

/** The 76dp cover: centre-cropped, no fade; a `secondary` panel with a glyph when it cannot load. */
function Cover({ uri }: { uri: string }) {
  const [failed, setFailed] = useState(false);
  const showPhoto = uri !== '' && !failed;
  return (
    <View
      style={{
        width: COVER,
        height: COVER,
        borderRadius: radius.md,
        backgroundColor: color.secondary,
        overflow: 'hidden',
        alignItems: 'center',
        justifyContent: 'center',
      }}
    >
      {showPhoto ? (
        <Image
          source={{ uri }}
          style={{ width: COVER, height: COVER }}
          contentFit="cover"
          onError={() => setFailed(true)}
        />
      ) : (
        <Ionicons name="image-outline" size={24} color={color.muted} />
      )}
    </View>
  );
}

/**
 * One post in the Diary (04-UI-SPEC 3.2, the developer's picture 07): the cover at the left, then
 * 'Oct 8 · Meal' (and '· Late' on a late post), the title in the light serif, the stars with the
 * figure, and the note in quotation marks when there is one. The whole row is one button; a
 * pressed row shows the `secondary` fill. A hairline sits under it, inset by the side margins.
 */
export function DiaryEntryRow({
  entry,
  onPress,
  onMore,
}: {
  entry: Entry;
  onPress: () => void;
  /** The three dots at the row's right: the same menu as on the entry page. */
  onMore: () => void;
}) {
  const note = quotedNote(entry.comment);
  return (
    <Pressable
      onPress={onPress}
      accessibilityRole="button"
      accessibilityLabel={rowSpoken(entry)}
      accessibilityHint="Opens the entry."
      style={({ pressed }) => ({
        paddingHorizontal: space.lg,
        backgroundColor: pressed ? color.secondary : 'transparent',
      })}
    >
      <View
        style={{
          minHeight: ROW_MIN_HEIGHT,
          flexDirection: 'row',
          alignItems: 'flex-start',
          paddingVertical: space.md,
          borderBottomWidth: 1,
          borderBottomColor: color.divider,
        }}
      >
        <Cover uri={entry.photo_url ?? ''} />
        <View style={{ flex: 1, marginLeft: space.md }}>
          <View style={{ flexDirection: 'row' }}>
            <Text style={labelStyle}>{datePrefix(entry.local_date)}</Text>
            <Text numberOfLines={1} ellipsizeMode="tail" style={{ ...labelStyle, flexShrink: 1 }}>
              {entry.category.name}
            </Text>
            {entry.late ? <Text style={labelStyle}>{' · Late'}</Text> : null}
          </View>
          <AppText role="note" numberOfLines={2} ellipsizeMode="tail">
            {entry.title}
          </AppText>
          <View style={{ marginTop: space.xs, flexDirection: 'row', alignItems: 'center' }}>
            <StarDisplay value={entry.stars} size={16} decorative />
            <View style={{ marginLeft: space.xs }}>
              <AppText role="label" tone="muted">
                {starsFigure(entry.stars)}
              </AppText>
            </View>
          </View>
          {note !== null ? (
            <View style={{ marginTop: space.xs }}>
              <Text
                numberOfLines={1}
                ellipsizeMode="tail"
                style={{
                  // The picture's note is a small italic serif: the serif of the titles at body size.
                  fontFamily: type.note.fontFamily,
                  fontSize: type.body.fontSize,
                  lineHeight: type.body.lineHeight,
                  fontStyle: 'italic',
                  color: color.muted,
                }}
              >
                {note}
              </Text>
            </View>
          ) : null}
        </View>
        <Pressable
          onPress={onMore}
          accessibilityRole="button"
          accessibilityLabel={`More for ${entry.title}`}
          hitSlop={8}
          style={{
            width: minTouchTarget,
            height: type.label.lineHeight + space.sm,
            marginRight: -space.sm,
            alignItems: 'center',
            justifyContent: 'flex-start',
          }}
        >
          <Ionicons name="ellipsis-horizontal" size={20} color={color.ink} />
        </Pressable>
      </View>
    </Pressable>
  );
}

/**
 * A skipped day (04-UI-SPEC 3.3, shown only under All): one quiet line, the date and the
 * category word struck through, as Today strikes the word of a skipped day. Not a button.
 */
export function DiarySkippedRow({ localDate, categoryName }: { localDate: string; categoryName: string }) {
  return (
    <View accessible accessibilityLabel={skippedSpoken(localDate, categoryName)} style={{ paddingHorizontal: space.lg }}>
      <View
        style={{
          minHeight: SKIPPED_MIN_HEIGHT,
          flexDirection: 'row',
          alignItems: 'center',
          paddingVertical: space.md,
          borderBottomWidth: 1,
          borderBottomColor: color.divider,
        }}
      >
        <Text style={labelStyle}>{datePrefix(localDate)}</Text>
        <Text
          numberOfLines={1}
          ellipsizeMode="tail"
          style={{ ...labelStyle, flexShrink: 1, textDecorationLine: 'line-through' }}
        >
          {categoryName}
        </Text>
      </View>
    </View>
  );
}

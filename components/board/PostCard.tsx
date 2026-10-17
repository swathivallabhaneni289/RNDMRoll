import { useState } from 'react';
import { Pressable, Text, View } from 'react-native';
import { Image } from 'expo-image';
import { Ionicons } from '@expo/vector-icons';
import { StarDisplay } from '@/components/entry/StarDisplay';
import { SizedAvatar } from '@/components/profile/SizedAvatar';
import { AppText } from '@/components/ui/AppText';
import type { Card } from '@/lib/api/board';
import { firstName, type Person } from '@/lib/api/social';
import { cardSpoken, starsFigure } from '@/lib/board/text';
import { PressScale } from '@/lib/motion/primitives';
import { color, radius, space, type } from '@/lib/theme/tokens';

/** The photo is 7 wide to 8 high, as the picture draws it. */
const PHOTO_ASPECT = 7 / 8;
const CHIP_HEIGHT = 28;
const CHIP_AVATAR = 22;
const STAR_SIZE = 12;

/** Small capitals as the Label role draws them. */
export const capsText = {
  fontFamily: type.label.fontFamily,
  fontSize: type.label.fontSize,
  lineHeight: type.label.lineHeight,
  letterSpacing: 1.5,
  textTransform: 'uppercase',
} as const;

/**
 * The name chip over a photo (05-UI-SPEC 3.4): a light pill with the round initial and the first
 * name ('You' on your own). A drawn label, not a control: the card it sits on is the button.
 */
export function NameChip({ person, label }: { person: Person | null; label: string }) {
  return (
    <View
      accessible={false}
      style={{
        height: CHIP_HEIGHT,
        borderRadius: radius.full,
        backgroundColor: color.dominant,
        flexDirection: 'row',
        alignItems: 'center',
        alignSelf: 'flex-start',
        paddingLeft: space.xs,
        paddingRight: space.sm,
        maxWidth: '100%',
      }}
    >
      <SizedAvatar
        size={CHIP_AVATAR}
        name={person ? person.name || person.username : label}
        uri={person?.photo_url ?? null}
        placeholder="initials"
      />
      <View style={{ marginLeft: space.xs, flexShrink: 1 }}>
        <AppText role="label" numberOfLines={1}>
          {label}
        </AppText>
      </View>
    </View>
  );
}

/**
 * One card of the Board (05-UI-SPEC 3.3, the developer's picture 09): the photo with the name chip
 * at its top left and the category at its bottom left, then the title and the stars with the figure.
 * The whole card is one button. Plain and light: no blur, no fade.
 */
export function PostCard({
  card,
  width,
  onPress,
  onOpenPerson,
}: {
  card: Card;
  width: number;
  onPress: () => void;
  /** A tap on the name chip opens the author's profile. */
  onOpenPerson?: (userId: string) => void;
}) {
  const [failed, setFailed] = useState(false);
  const name = card.mine ? 'You' : firstName(card.author);
  const height = Math.round(width / PHOTO_ASPECT);

  return (
    <PressScale
      onPress={onPress}
      accessibilityRole="button"
      accessibilityLabel={cardSpoken({
        title: card.title,
        stars: card.stars,
        category: card.category.name,
        by: card.mine ? 'you' : name,
        late: card.late,
      })}
      accessibilityHint="Opens the post."
      style={{ width }}
    >
      <View style={{ width, height, backgroundColor: color.secondary }}>
        {card.photo_url !== '' && !failed ? (
          <Image
            source={{ uri: card.photo_url }}
            style={{ width, height }}
            contentFit="cover"
            transition={0}
            onError={() => setFailed(true)}
          />
        ) : (
          <View style={{ flex: 1, alignItems: 'center', justifyContent: 'center' }}>
            <Ionicons name="image-outline" size={28} color={color.muted} />
          </View>
        )}
        <View style={{ position: 'absolute', top: space.sm, left: space.sm, right: space.sm }}>
          {onOpenPerson ? (
            <Pressable
              onPress={() => onOpenPerson(card.author.id)}
              accessibilityRole="button"
              accessibilityLabel={card.mine ? 'Your profile' : `${name}'s profile`}
              hitSlop={6}
              style={{ alignSelf: 'flex-start', maxWidth: '100%' }}
            >
              <NameChip person={card.author} label={name} />
            </Pressable>
          ) : (
            <NameChip person={card.author} label={name} />
          )}
        </View>
        {card.category.name !== '' ? (
          <View
            style={{
              position: 'absolute',
              left: space.sm,
              bottom: space.sm,
              maxWidth: width - 2 * space.sm,
              backgroundColor: color.dominant,
              borderRadius: radius.sm,
              paddingHorizontal: space.sm,
              paddingVertical: space.xs,
            }}
          >
            <Text numberOfLines={1} style={{ ...capsText, color: color.ink }}>
              {card.category.name}
            </Text>
          </View>
        ) : null}
      </View>
      <View style={{ marginTop: space.sm }}>
        <Text
          numberOfLines={1}
          ellipsizeMode="tail"
          maxFontSizeMultiplier={1.3}
          style={{
            fontFamily: type.note.fontFamily,
            fontSize: type.body.fontSize,
            lineHeight: type.body.lineHeight,
            color: color.ink,
          }}
        >
          {card.title}
        </Text>
      </View>
      <View style={{ marginTop: space.xs, flexDirection: 'row', alignItems: 'center' }}>
        <StarDisplay value={card.stars} size={STAR_SIZE} decorative />
        <View style={{ marginLeft: space.xs }}>
          <AppText role="label" tone="muted">
            {card.late ? `${starsFigure(card.stars)}  Late` : starsFigure(card.stars)}
          </AppText>
        </View>
      </View>
    </PressScale>
  );
}

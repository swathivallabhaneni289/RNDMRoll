import { Pressable, Text, View, type AccessibilityState } from 'react-native';
import { color, minTouchTarget, radius, space, type } from '@/lib/theme/tokens';

/**
 * The one shared chip (04-UI-SPEC section 2): a pill with a short label and, optionally, a figure
 * at the right. The developer's pictures draw the Diary's category chips, the Profile page's taste
 * chips and the Edit button as pills, so the round shape is made ONCE, here. This is the only file
 * besides the avatar, the icon badge and the Spin button that names `radius.full`; every screen
 * uses `Chip` and never rounds a tappable on its own.
 *
 * Tones: `selected` (ink fill, light text), `soft` (the soft field fill, no border), `outline`
 * (a 1dp faint border) and `strong` (a 1dp ink border, the Edit button). `height` is the visible
 * height; the touch target is topped up to 44dp with `hitSlop`. Without `onPress` the chip is
 * plain, not a button (a taste chip until the Diary can filter by it). Nothing animates.
 */
export type ChipTone = 'selected' | 'soft' | 'outline' | 'strong';

type ChipProps = {
  label: string;
  /** A figure at the right, drawn in the serif ("4.6"). */
  value?: string;
  tone: ChipTone;
  onPress?: () => void;
  /** Visible height: 32 (the default), 36 or 40. */
  height?: number;
  accessibilityLabel?: string;
  accessibilityHint?: string;
  accessibilityState?: AccessibilityState;
};

const SIDE_PADDING = space.md;

const toneStyle: Record<ChipTone, { fill: string; border: string | null; text: string }> = {
  selected: { fill: color.ink, border: null, text: color.dominant },
  soft: { fill: color.fieldSoft, border: null, text: color.ink },
  outline: { fill: color.dominant, border: color.inkRest, text: color.ink },
  strong: { fill: color.dominant, border: color.ink, text: color.ink },
};

export function Chip({
  label,
  value,
  tone,
  onPress,
  height = 32,
  accessibilityLabel,
  accessibilityHint,
  accessibilityState,
}: ChipProps) {
  const look = toneStyle[tone];
  const slop = Math.max(0, (minTouchTarget - height) / 2);
  const hasValue = value !== undefined;

  const body = (pressed: boolean) => (
    <View
      style={{
        minHeight: height,
        borderRadius: radius.full,
        borderWidth: look.border ? 1 : 0,
        borderColor: look.border ?? 'transparent',
        backgroundColor: pressed && tone !== 'selected' ? color.secondary : look.fill,
        paddingHorizontal: SIDE_PADDING,
        flexDirection: 'row',
        alignItems: 'center',
        justifyContent: hasValue ? 'space-between' : 'center',
      }}
    >
      <Text
        numberOfLines={1}
        ellipsizeMode="tail"
        style={{
          flexShrink: 1,
          fontFamily: type.body.fontFamily,
          fontSize: type.body.fontSize,
          lineHeight: type.body.lineHeight,
          color: look.text,
        }}
      >
        {label}
      </Text>
      {hasValue ? (
        <Text
          numberOfLines={1}
          style={{
            marginLeft: space.sm,
            // The figures are drawn in the serif, as in the developer's picture.
            fontFamily: type.note.fontFamily,
            fontSize: type.body.fontSize,
            lineHeight: type.body.lineHeight,
            color: look.text,
          }}
        >
          {value}
        </Text>
      ) : null}
    </View>
  );

  if (!onPress) {
    return (
      <View
        accessible
        accessibilityLabel={accessibilityLabel ?? (hasValue ? `${label}, ${value}` : label)}
        accessibilityState={accessibilityState}
      >
        {body(false)}
      </View>
    );
  }

  return (
    <Pressable
      onPress={onPress}
      hitSlop={{ top: slop, bottom: slop, left: 0, right: 0 }}
      accessibilityRole="button"
      accessibilityLabel={accessibilityLabel ?? (hasValue ? `${label}, ${value}` : label)}
      accessibilityHint={accessibilityHint}
      accessibilityState={accessibilityState}
    >
      {({ pressed }) => body(pressed)}
    </Pressable>
  );
}

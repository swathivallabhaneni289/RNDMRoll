import { Pressable, View } from 'react-native';
import { Ionicons } from '@expo/vector-icons';
import { Circle, Svg } from 'react-native-svg';
import { color, minTouchTarget } from '@/lib/theme/tokens';

const GLYPH = 24;
/** The dot with its light ring, drawn as a shape (no rounded view). */
const DOT = 8;
const RING = 2;
const MARK = DOT + RING * 2;

/** The solid unread dot, used on the bell and on the rows of Notifications. */
export function UnreadDot({ ring = false }: { ring?: boolean }) {
  const size = ring ? MARK : DOT;
  return (
    <Svg width={size} height={size} pointerEvents="none">
      {ring ? <Circle cx={size / 2} cy={size / 2} r={size / 2} fill={color.dominant} /> : null}
      <Circle cx={size / 2} cy={size / 2} r={DOT / 2} fill={color.ink} />
    </Svg>
  );
}

/**
 * The bell of the Spin page and the Board (05-UI-SPEC 9.1): the outline bell in a 44dp target, with
 * a small solid dot at its top right while anything is unread. No number, no colour, no motion.
 */
export function BellButton({ unread, onPress }: { unread: boolean; onPress: () => void }) {
  return (
    <Pressable
      onPress={onPress}
      accessibilityRole="button"
      accessibilityLabel={unread ? 'Notifications, new' : 'Notifications'}
      accessibilityHint="Opens your notifications."
      hitSlop={4}
      style={{ width: minTouchTarget, height: minTouchTarget, alignItems: 'flex-end', justifyContent: 'center' }}
    >
      <View style={{ width: GLYPH, height: GLYPH }}>
        <Ionicons name="notifications-outline" size={GLYPH} color={color.ink} />
        {unread ? (
          <View style={{ position: 'absolute', top: -RING, right: -RING }}>
            <UnreadDot ring />
          </View>
        ) : null}
      </View>
    </Pressable>
  );
}

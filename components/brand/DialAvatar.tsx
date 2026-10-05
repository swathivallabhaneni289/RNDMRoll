import { View } from 'react-native';
import { Image } from 'expo-image';
import { Circle, Path, Svg } from 'react-native-svg';
import { AppText } from '@/components/ui/AppText';
import { color, radius } from '@/lib/theme/tokens';

const DISC = 104;
const RING_R = 64;
const SIZE = RING_R * 2;

function polar(r: number, deg: number) {
  const a = (deg * Math.PI) / 180;
  return `${(RING_R + r * Math.sin(a)).toFixed(2)} ${(RING_R - r * Math.cos(a)).toFixed(2)}`;
}

/** Sixty dial ticks just inside the ring, longer every fifth, the same dial as the wheel background. */
const TICKS = (() => {
  let d = '';
  for (let k = 0; k < 60; k++) {
    const r0 = RING_R - 3;
    const r1 = r0 - (k % 5 === 0 ? 6 : 3);
    d += `M ${polar(r0, k * 6)} L ${polar(r1, k * 6)} `;
  }
  return d;
})();

/** First letter of the first and last word, so "Ava Stone" gives "AS". Splits by code point, not by UTF-16 unit. */
function initialsOf(name: string) {
  const words = name.trim().split(/\s+/).filter(Boolean);
  if (words.length === 0) return '';
  const first = Array.from(words[0])[0] ?? '';
  const last = words.length > 1 ? Array.from(words[words.length - 1])[0] ?? '' : '';
  return (first + last).toUpperCase();
}

/**
 * The profile avatar: a round photo, or the person's initials in the display serif when there is
 * no photo yet, inside a thin dial ring. The ring is the one piece of the wheel's look this
 * screen borrows; it is drawn, static and ink at low opacity.
 */
export function DialAvatar({ name, uri }: { name: string; uri?: string | null }) {
  return (
    <View style={{ width: SIZE, height: SIZE, alignItems: 'center', justifyContent: 'center' }}>
      <Svg width={SIZE} height={SIZE} pointerEvents="none" style={{ position: 'absolute' }}>
        <Circle cx={RING_R} cy={RING_R} r={RING_R - 0.75} fill="none" stroke={color.ink} strokeOpacity={0.18} strokeWidth={1.5} />
        <Path d={TICKS} stroke={color.ink} strokeOpacity={0.3} strokeWidth={1} fill="none" />
      </Svg>
      <View
        style={{
          width: DISC,
          height: DISC,
          borderRadius: radius.full,
          backgroundColor: color.secondary,
          alignItems: 'center',
          justifyContent: 'center',
          overflow: 'hidden',
        }}
      >
        {uri ? (
          <Image source={{ uri }} style={{ width: DISC, height: DISC }} contentFit="cover" accessibilityLabel="Profile photo" />
        ) : (
          <View accessibilityElementsHidden importantForAccessibility="no-hide-descendants">
            <AppText role="display" allowFontScaling={false}>
              {initialsOf(name)}
            </AppText>
          </View>
        )}
      </View>
    </View>
  );
}

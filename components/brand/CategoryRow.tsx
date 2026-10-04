import { useEffect } from 'react';
import { View } from 'react-native';
import Animated, { Easing, useAnimatedStyle, useSharedValue, withTiming } from 'react-native-reanimated';
import { Circle, Path, Rect, Svg } from 'react-native-svg';
import { color as tokenColor } from '@/lib/theme/tokens';

/**
 * The wheel categories as thin ink line drawings (not emoji), in the order of the default
 * wheel (movie, book, song, meal, place) with camera added at the end. Each is drawn on a 40 by 40 grid with round caps and no
 * fill, so they sit beside the wheel logo as one family. They are first-pass drawings: the
 * developer can redraw any of them by replacing its paths.
 */
export const CATEGORY_ORDER = ['MOVIE', 'BOOK', 'SONG', 'MEAL', 'PLACE', 'CAMERA'] as const;
export type Category = (typeof CATEGORY_ORDER)[number];

/** The wheel's eight sections, clockwise from the first light wedge (a sample: two categories repeat). */
export const WHEEL_SECTORS: readonly Category[] = ['MOVIE', 'BOOK', 'SONG', 'MEAL', 'PLACE', 'CAMERA', 'SONG', 'PLACE'];

export function CategoryIcon({ word, size = 40, color = tokenColor.ink }: { word: Category; size?: number; color?: string }) {
  return (
    <Svg width={size} height={size} viewBox="0 0 40 40" fill="none" stroke={color} strokeWidth={1.7} strokeLinecap="round" strokeLinejoin="round">
      {word === 'MOVIE' && (
        <>
          <Rect x={5} y={9} width={30} height={21} rx={3} />
          <Path d="M17 14.5 L17 24.5 L26 19.5 Z" />
          <Path d="M14 34 H26" />
        </>
      )}
      {word === 'BOOK' && (
        <>
          <Path d="M20 11 C15.5 8 9.5 8 5 9.5 V30 C9.5 28.5 15.5 28.5 20 31.5 C24.5 28.5 30.5 28.5 35 30 V9.5 C30.5 8 24.5 8 20 11 Z" />
          <Path d="M20 11 V31.5" />
        </>
      )}
      {word === 'SONG' && (
        <>
          <Circle cx={12.5} cy={29} r={3.6} fill={color} />
          <Circle cx={28.5} cy={26} r={3.6} fill={color} />
          <Path d="M16.1 29 V11.5 L32.1 8 V26" />
          <Path d="M16.1 16.5 L32.1 13" />
        </>
      )}
      {word === 'MEAL' && (
        <>
          <Circle cx={21} cy={20} r={8.5} />
          <Circle cx={21} cy={20} r={5} />
          <Path d="M4.5 9 V15.5 M7.5 9 V15.5 M10.5 9 V15.5 M4.5 15.5 Q7.5 19 10.5 15.5 M7.5 18.5 V32" />
          <Path d="M35.5 9 C32 12 31.5 17 33 21.5 H35.5 V9 Z M35.5 21.5 V32" />
        </>
      )}
      {word === 'CAMERA' && (
        <>
          <Rect x={4} y={12} width={32} height={20} rx={3} />
          <Path d="M13.5 12 L16 8 H24 L26.5 12" />
          <Circle cx={20} cy={22} r={5.5} />
          <Circle cx={30.5} cy={16.5} r={0.9} fill={color} />
        </>
      )}
      {word === 'PLACE' && (
        <>
          <Path d="M5 28 H35" />
          <Path d="M11 28 A9 9 0 0 1 29 28" />
          <Path d="M20 15.5 V10.5 M28.5 19.5 L31.5 16.5 M11.5 19.5 L8.5 16.5 M31.5 24 L35 23 M8.5 24 L5 23" />
          <Path d="M10 33 H17 M23 33 H30" />
        </>
      )}
    </Svg>
  );
}

function Item({ word, size, active }: { word: Category; size: number; active: boolean }) {
  const k = useSharedValue(0);
  useEffect(() => {
    k.value = withTiming(active ? 1 : 0, { duration: 450, easing: Easing.out(Easing.cubic) });
  }, [active, k]);
  const style = useAnimatedStyle(() => ({
    opacity: 0.32 + 0.68 * k.value,
    transform: [{ translateY: -5 * k.value }, { scale: 1 + 0.12 * k.value }],
  }));
  return (
    <Animated.View style={style} accessibilityElementsHidden importantForAccessibility="no-hide-descendants">
      <CategoryIcon word={word} size={size} />
    </Animated.View>
  );
}

/** A row of the drawings; the one named by `active` lifts and darkens, the rest stay faint. */
export function CategoryRow({ active = null, size = 40 }: { active?: string | null; size?: number }) {
  return (
    <View style={{ flexDirection: 'row', justifyContent: 'center', gap: 14 }}>
      {CATEGORY_ORDER.map((w) => (
        <Item key={w} word={w} size={size} active={active === w} />
      ))}
    </View>
  );
}

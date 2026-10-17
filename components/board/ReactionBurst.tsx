import { useEffect, useMemo } from 'react';
import { StyleSheet, Text, useWindowDimensions, View } from 'react-native';
import Animated, { Easing, interpolate, useAnimatedStyle, useSharedValue, withDelay, withTiming } from 'react-native-reanimated';

const COUNT = 8;
const RISE_MS = 1300;
/** How long the layer stays before the caller may drop it: the last start plus one rise. */
export const BURST_MS = RISE_MS + COUNT * 70 + 120;

type Flyer = { x: number; size: number; delay: number; drift: number; rise: number };

function Particle({ char, flyer, bottom }: { char: string; flyer: Flyer; bottom: number }) {
  const p = useSharedValue(0);
  useEffect(() => {
    p.value = withDelay(flyer.delay, withTiming(1, { duration: RISE_MS, easing: Easing.out(Easing.quad) }));
    // Mount-only: a new burst is a new key, so each copy runs once.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
  const animStyle = useAnimatedStyle(() => ({
    opacity: interpolate(p.value, [0, 0.08, 0.65, 1], [0, 1, 1, 0]),
    transform: [
      { translateY: -p.value * flyer.rise },
      { translateX: p.value * flyer.drift },
      { scale: interpolate(p.value, [0, 0.2, 1], [0.4, 1.15, 1]) },
    ],
  }));
  return (
    <Animated.View style={[{ position: 'absolute', left: flyer.x, bottom }, animStyle]}>
      <Text allowFontScaling={false} style={{ fontSize: flyer.size, lineHeight: flyer.size * 1.2 }}>
        {char}
      </Text>
    </Animated.View>
  );
}

/**
 * The reaction you just sent, flown up the screen in a small spray of copies (developer,
 * 2026-10-17: like a story reaction on Instagram). Drawn over the whole page, never takes a touch;
 * the caller mounts it with a fresh `key` for every tap and leaves it out under Reduce Motion.
 */
export function ReactionBurst({ char }: { char: string }) {
  const { width, height } = useWindowDimensions();
  const flyers = useMemo<Flyer[]>(
    () =>
      Array.from({ length: COUNT }, (_, i) => {
        const size = 34 + Math.round(Math.random() * 26);
        return {
          size,
          x: width * 0.12 + Math.random() * (width * 0.76 - size),
          delay: i * 70 + Math.round(Math.random() * 60),
          drift: (Math.random() - 0.5) * 60,
          rise: height * (0.45 + Math.random() * 0.2),
        };
      }),
    [width, height],
  );
  return (
    <View pointerEvents="none" style={StyleSheet.absoluteFill}>
      {flyers.map((flyer, i) => (
        <Particle key={i} char={char} flyer={flyer} bottom={height * 0.12} />
      ))}
    </View>
  );
}

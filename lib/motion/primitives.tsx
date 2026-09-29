import { ReactNode, useEffect } from 'react';
import { Pressable, PressableProps, ViewStyle } from 'react-native';
import Animated, {
  Easing,
  useAnimatedStyle,
  useSharedValue,
  withDelay,
  withSpring,
  withTiming,
} from 'react-native-reanimated';

/**
 * Shared animation primitives for docs/motion-interaction-direction.md Section 10.
 * Deliberately covers only what Section 1 (onboarding) needs today. WheelSettle,
 * RatingSelect, StaggeredList, and ImageExpand aren't built here: nothing in the
 * app yet has a wheel, a rating control, a scrolling list, or a detail-view
 * expansion to drive them, and every one of those has real interaction/data
 * questions (settle curve against real wheel physics, list virtualization
 * against real diary data) that a speculative build now would just guess at.
 * Add each when the phase that needs it actually gets planned.
 */

const EASE_OUT = Easing.out(Easing.cubic);

function useEntranceProgress(delayMs: number, durationMs: number) {
  const progress = useSharedValue(0);
  useEffect(() => {
    progress.value = withDelay(delayMs, withTiming(1, { duration: durationMs, easing: EASE_OUT }));
    // Mount-only: an entrance animation replays by remounting the component
    // (a fresh key), not by re-running this effect on prop change.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
  return progress;
}

type EntranceProps = {
  children?: ReactNode;
  delay?: number;
  duration?: number;
  style?: ViewStyle;
};

/**
 * Heading/supporting-text entrance: fade + rise. Distance defaults to 14px,
 * the middle of the spec's 12-16px range, rather than a library-default
 * offset large enough to read as a slide.
 */
export function FadeInUp({ children, delay = 0, duration = 400, distance = 14, style }: EntranceProps & { distance?: number }) {
  const progress = useEntranceProgress(delay, duration);
  const animStyle = useAnimatedStyle(() => ({
    opacity: progress.value,
    transform: [{ translateY: (1 - progress.value) * distance }],
  }));
  return <Animated.View style={[style, animStyle]}>{children}</Animated.View>;
}

/** Semantic alias: RevealText is FadeInUp used specifically for a text block. */
export const RevealText = FadeInUp;

/** Image/visual entrance: fade + the spec's 0.97 -> 1.0 scale, no vertical movement. */
export function ImageReveal({ children, delay = 0, duration = 450, style }: EntranceProps) {
  const progress = useEntranceProgress(delay, duration);
  const animStyle = useAnimatedStyle(() => ({
    opacity: progress.value,
    transform: [{ scale: 0.97 + progress.value * 0.03 }],
  }));
  return <Animated.View style={[style, animStyle]}>{children}</Animated.View>;
}

/**
 * Press feedback for any tappable element: compress slightly on press-in,
 * settle back with a spring (no overshoot) on release. Wraps its own
 * Pressable rather than taking one, so the scale transform and the touch
 * target move together.
 */
export function PressScale({
  children,
  onPress,
  style,
  scaleTo = 0.97,
  ...rest
}: Omit<PressableProps, 'style'> & { children?: ReactNode; style?: ViewStyle; scaleTo?: number }) {
  const scale = useSharedValue(1);
  const animStyle = useAnimatedStyle(() => ({ transform: [{ scale: scale.value }] }));
  return (
    // `style` (layout/position, e.g. an absolute-positioned chevron) lives on the
    // Pressable itself, same as a plain Pressable would take it; the scale
    // transform is a separate inner wrapper so a positioned caller's layout is
    // never at the mercy of an animated child collapsing to its own box.
    <Pressable
      onPress={onPress}
      onPressIn={() => {
        scale.value = withTiming(scaleTo, { duration: 90, easing: EASE_OUT });
      }}
      onPressOut={() => {
        scale.value = withSpring(1, { damping: 16, stiffness: 220, overshootClamping: true });
      }}
      style={style}
      {...rest}
    >
      <Animated.View style={animStyle}>{children}</Animated.View>
    </Pressable>
  );
}

/**
 * Computes a staggered delay for the Nth element of a sequence. `base` is
 * where the sequence starts (e.g. after a screen transition resolves);
 * `step` is the gap between elements, defaulting to the middle of the
 * spec's 60-120ms range.
 */
export function staggerDelay(index: number, base = 0, step = 90) {
  return base + index * step;
}

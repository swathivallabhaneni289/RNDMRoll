import { Platform, StyleSheet } from 'react-native';
import Animated, { interpolateColor, useAnimatedStyle } from 'react-native-reanimated';
import { useFocusProgress } from '@/lib/motion/navigation';
import { color, type } from '@/lib/theme/tokens';

/** The two ends of the glide, read once so the animation holds plain strings. */
const MUTED = color.muted;
const INK = color.ink;

/**
 * What the bar itself speaks for a tab with a plain text label on iOS ("Home, tab, 1 of 5").
 * It stops doing that once `tabBarLabel` is a function, so the same sentence is handed over
 * through `tabBarAccessibilityLabel`. Other platforms get nothing, as before.
 */
export function spokenTabLabel(
  navigation: { getState(): { routes: ReadonlyArray<{ key: string }> } },
  route: { key: string },
  title: string,
): string | undefined {
  if (Platform.OS !== 'ios') return undefined;
  const { routes } = navigation.getState();
  const index = routes.findIndex((r) => r.key === route.key);
  return `${title}, tab, ${index + 1} of ${routes.length}`;
}

/**
 * One bottom-bar label (Phase 2.2, D-03). Its colour glides between `muted` and `ink` over
 * `barColorMs` as the tab is selected and deselected; the weight stays regular, so the label's
 * width never changes.
 *
 * When `tabBarLabel` is a function the bar stops applying its own label text style, so the
 * pieces it used to apply are set here: the app's `type.label` at weight 400 (the same as
 * `tabBarLabelStyle` in the layout), one line, centred, no font scaling.
 */
export function TabLabel({ title, focused }: { title: string; focused: boolean }) {
  const progress = useFocusProgress(focused);

  const colorStyle = useAnimatedStyle(() => ({
    color: interpolateColor(progress.value, [0, 1], [MUTED, INK]),
  }));

  return (
    <Animated.Text numberOfLines={1} allowFontScaling={false} style={[styles.label, colorStyle]}>
      {title}
    </Animated.Text>
  );
}

const styles = StyleSheet.create({
  label: {
    textAlign: 'center',
    backgroundColor: 'transparent',
    fontFamily: type.label.fontFamily,
    fontSize: type.label.fontSize,
    lineHeight: type.label.lineHeight,
    fontWeight: '400',
  },
});

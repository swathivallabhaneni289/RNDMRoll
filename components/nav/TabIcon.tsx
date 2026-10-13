import { Ionicons } from '@expo/vector-icons';
import { ComponentProps } from 'react';
import { StyleSheet, View } from 'react-native';
import Animated, { useAnimatedStyle } from 'react-native-reanimated';
import { useFocusProgress } from '@/lib/motion/navigation';
import { color } from '@/lib/theme/tokens';

/** The size the bar's icons have always had here (the bar itself would hand over 25). */
const ICON_SIZE = 24;

type IconName = ComponentProps<typeof Ionicons>['name'];

type NavigationLike = { getState(): { index: number; routes: ReadonlyArray<{ key: string }> } };

/**
 * Whether `route` is the tab selected in its own tab navigator, read from that navigator's state.
 *
 * `TabIcon` has to be given this and not the `focused` that the `tabBarIcon` callback receives:
 * the bar draws every icon twice, one copy with `focused` fixed to true and one fixed to false,
 * and flips which of the two is visible at once, so that `focused` never changes for either copy.
 * `navigation.isFocused()` is not the same thing either: it also turns false while a page covers
 * the whole tab group, which would replay the glide on the way back.
 */
export function isSelectedTab(navigation: NavigationLike, route: { key: string }): boolean {
  const state = navigation.getState();
  return state.routes[state.index]?.key === route.key;
}

/**
 * One bottom-bar icon (Phase 2.2, D-03). The outline glyph is drawn in `muted` and again in `ink`
 * exactly on top of it; the `ink` copy fades in over `barColorMs` when the tab becomes selected
 * and out when it stops being selected. The two copies are the same glyph, so a cross-fade is a
 * straight blend of the two colours and the outline never changes shape or weight.
 *
 * The `muted` copy stays in normal flow and gives the component its size, so nothing in the bar
 * moves. It is switched off once the glide has fully arrived, so a selected icon at rest is the
 * single `ink` glyph and an unselected one is the single `muted` glyph, as before this component.
 */
export function TabIcon({ name, focused }: { name: IconName; focused: boolean }) {
  const progress = useFocusProgress(focused);

  const mutedStyle = useAnimatedStyle(() => ({ opacity: progress.value >= 1 ? 0 : 1 }));
  const inkStyle = useAnimatedStyle(() => ({ opacity: progress.value }));

  return (
    <View>
      <Animated.View style={mutedStyle}>
        <Ionicons name={name} size={ICON_SIZE} color={color.muted} />
      </Animated.View>
      <Animated.View style={[StyleSheet.absoluteFill, inkStyle]}>
        <Ionicons name={name} size={ICON_SIZE} color={color.ink} />
      </Animated.View>
    </View>
  );
}

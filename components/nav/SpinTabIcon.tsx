import { StyleSheet, View } from 'react-native';
import Animated, { useAnimatedStyle } from 'react-native-reanimated';
import { BrandMark } from '@/components/brand/BrandMark';
import { useFocusProgress } from '@/lib/motion/navigation';
import { color } from '@/lib/theme/tokens';

/** The same size as the other bar icons. */
const ICON_SIZE = 24;

/**
 * The Spin tab's icon: the app's wheel mark, drawn like the other tabs (developer, 2026-10-16: no
 * raised circle, the Spin tab sits in the bar like Diary, Board and Profile). The mark is drawn in
 * `muted` and again in `ink` on top, and the `ink` copy glides in when the tab is selected, the
 * same way `TabIcon` does it. `focused` must come from `isSelectedTab`, as for `TabIcon`.
 */
export function SpinTabIcon({ focused }: { focused: boolean }) {
  const progress = useFocusProgress(focused);

  const mutedStyle = useAnimatedStyle(() => ({ opacity: progress.value >= 1 ? 0 : 1 }));
  const inkStyle = useAnimatedStyle(() => ({ opacity: progress.value }));

  return (
    <View>
      <Animated.View style={mutedStyle}>
        <BrandMark size={ICON_SIZE} color={color.muted} wedgeColor={color.dominant} />
      </Animated.View>
      <Animated.View style={[StyleSheet.absoluteFill, inkStyle]}>
        <BrandMark size={ICON_SIZE} color={color.ink} wedgeColor={color.dominant} />
      </Animated.View>
    </View>
  );
}

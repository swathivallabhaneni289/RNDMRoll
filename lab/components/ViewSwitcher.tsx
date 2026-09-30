import { useEffect, useRef, useState } from 'react';
import { LayoutChangeEvent, Pressable, View } from 'react-native';
import Animated, {
  interpolate,
  useAnimatedStyle,
  useReducedMotion,
  useSharedValue,
  withTiming,
} from 'react-native-reanimated';
import { color, minTouchTarget, space, type } from '@/lib/theme/tokens';
import { duration, easeInOut } from '@/lab/motion/timing';

export type ViewMode = 'grid' | 'diary';

const MODES: ViewMode[] = ['grid', 'diary'];
const LABELS: Record<ViewMode, string> = { grid: 'GRID', diary: 'DIARY' };
// Ink at 0.6 on the warm ground is 4.8:1, the floor for 14dp text is 4.5:1; the
// underline carries the selected state, so the inactive label only has to recede.
const INACTIVE_OPACITY = 0.6;
const INDICATOR_HEIGHT = 2;

type Placement = { x: number; width: number };

type Props = {
  value: ViewMode;
  onChange: (mode: ViewMode) => void;
};

/**
 * GRID / DIARY as typography, not a control: two small-caps labels and a thin ink
 * underline that slides under the active one. The inactive label sits at 40%
 * opacity. No container, no fill, no pill. The indicator is placed from each
 * label's measured layout, so it always matches the real text width.
 */
export function ViewSwitcher({ value, onChange }: Props) {
  const reduced = useReducedMotion();
  const [placements, setPlacements] = useState<Partial<Record<ViewMode, Placement>>>({});
  const x = useSharedValue(0);
  const width = useSharedValue(0);
  const active = useSharedValue(value === 'diary' ? 1 : 0);
  const placed = useRef(false);

  useEffect(() => {
    const target = placements[value];
    if (!target) return;
    // The first placement snaps; every later move slides.
    const ms = placed.current && !reduced ? duration.base : 0;
    placed.current = true;
    x.value = withTiming(target.x, { duration: ms, easing: easeInOut });
    width.value = withTiming(target.width, { duration: ms, easing: easeInOut });
  }, [value, placements, reduced, x, width]);

  useEffect(() => {
    active.value = withTiming(value === 'diary' ? 1 : 0, { duration: reduced ? 0 : duration.base, easing: easeInOut });
  }, [value, reduced, active]);

  const indicatorStyle = useAnimatedStyle(() => ({
    width: width.value,
    transform: [{ translateX: x.value }],
  }));
  const gridLabelStyle = useAnimatedStyle(() => ({
    opacity: interpolate(active.value, [0, 1], [1, INACTIVE_OPACITY]),
  }));
  const diaryLabelStyle = useAnimatedStyle(() => ({
    opacity: interpolate(active.value, [0, 1], [INACTIVE_OPACITY, 1]),
  }));

  function record(mode: ViewMode, event: LayoutChangeEvent) {
    const { x: left, width: labelWidth } = event.nativeEvent.layout;
    setPlacements((current) => {
      const existing = current[mode];
      if (existing && existing.x === left && existing.width === labelWidth) return current;
      return { ...current, [mode]: { x: left, width: labelWidth } };
    });
  }

  return (
    <View accessibilityRole="tablist" style={{ flexDirection: 'row', gap: space.lg }}>
      {MODES.map((mode) => (
        <Pressable
          key={mode}
          onLayout={(event) => record(mode, event)}
          onPress={() => onChange(mode)}
          accessibilityRole="tab"
          accessibilityState={{ selected: value === mode }}
          accessibilityLabel={mode === 'grid' ? 'Grid view' : 'Diary view'}
          style={{ minHeight: minTouchTarget, justifyContent: 'center' }}
        >
          <Animated.Text
            style={[
              {
                fontFamily: type.button.fontFamily,
                fontSize: type.label.fontSize,
                lineHeight: type.label.lineHeight,
                letterSpacing: 1.2,
                color: color.ink,
              },
              mode === 'grid' ? gridLabelStyle : diaryLabelStyle,
            ]}
          >
            {LABELS[mode]}
          </Animated.Text>
        </Pressable>
      ))}
      <Animated.View
        style={[
          {
            position: 'absolute',
            left: 0,
            bottom: 6,
            height: INDICATOR_HEIGHT,
            backgroundColor: color.ink,
            pointerEvents: 'none',
          },
          indicatorStyle,
        ]}
      />
    </View>
  );
}

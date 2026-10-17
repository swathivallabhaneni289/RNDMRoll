import { useEffect, useRef, type ReactNode } from 'react';
import { Pressable, Text, View } from 'react-native';
import Animated, { useAnimatedStyle, useSharedValue, withSequence, withSpring, withTiming } from 'react-native-reanimated';
import { REACTIONS, type ReactionName } from '@/lib/entry/reactions';
import { EASE_OUT, PressScale } from '@/lib/motion/primitives';
import { useReduceMotion } from '@/lib/motion/useReduceMotion';
import { color, radius, type } from '@/lib/theme/tokens';

const BUTTON = 47;

/**
 * A quick grow and settle when a button becomes the chosen one (developer, 2026-10-17: a tap
 * should feel confirmed). Not on the first draw, so a reaction already saved stays still.
 */
function Pop({ selected, children }: { selected: boolean; children: ReactNode }) {
  const scale = useSharedValue(1);
  const was = useRef(selected);
  useEffect(() => {
    if (selected && !was.current) {
      scale.value = withSequence(
        withTiming(1.25, { duration: 120, easing: EASE_OUT }),
        withSpring(1, { damping: 12, stiffness: 260 }),
      );
    }
    was.current = selected;
  }, [selected, scale]);
  const animStyle = useAnimatedStyle(() => ({ transform: [{ scale: scale.value }] }));
  return <Animated.View style={animStyle}>{children}</Animated.View>;
}

/**
 * The six round reaction buttons (UI spec 6), spread over the full column. Unselected: no fill and
 * a 1dp `inkRest` outline. Selected: `secondary` fill and a 1.5dp `ink` ring, with a short pop as
 * it is chosen (none under Reduce Motion). Tapping the selected one clears it. The round shape is
 * one of the two named exceptions to the no-pill rule (the other is the Camera and Library pair in
 * PhotoField). The characters come from `lib/entry/reactions.ts`.
 */
export function ReactionPicker({
  value,
  onChange,
}: {
  value: ReactionName | null;
  onChange: (next: ReactionName | null) => void;
}) {
  const reduce = useReduceMotion();

  return (
    <View
      accessibilityRole="radiogroup"
      accessibilityLabel="Reaction, optional"
      style={{ flexDirection: 'row', justifyContent: 'space-between' }}
    >
      {REACTIONS.map((reaction) => {
        const selected = value === reaction.name;
        const face = (
          <View
            style={{
              width: BUTTON,
              height: BUTTON,
              borderRadius: radius.full,
              borderWidth: selected ? 1.5 : 1,
              borderColor: selected ? color.ink : color.inkRest,
              backgroundColor: selected ? color.secondary : 'transparent',
              alignItems: 'center',
              justifyContent: 'center',
            }}
          >
            <Text
              allowFontScaling={false}
              style={{ fontSize: type.heading.fontSize, lineHeight: type.heading.lineHeight }}
            >
              {reaction.char}
            </Text>
          </View>
        );
        const shared = {
          onPress: () => onChange(selected ? null : reaction.name),
          accessibilityRole: 'button' as const,
          accessibilityLabel: reaction.label,
          accessibilityState: { selected },
          accessibilityHint: selected ? 'Tap again to clear.' : undefined,
        };
        return reduce ? (
          <Pressable key={reaction.name} {...shared}>
            {face}
          </Pressable>
        ) : (
          <PressScale key={reaction.name} {...shared}>
            <Pop selected={selected}>{face}</Pop>
          </PressScale>
        );
      })}
    </View>
  );
}

import { ReactNode, useRef } from 'react';
import {
  GestureResponderEvent,
  PanResponder,
  PanResponderGestureState,
  Pressable,
  View,
} from 'react-native';
import { Ionicons } from '@expo/vector-icons';
import { color, minTouchTarget, space } from '@/lib/theme/tokens';

const SWIPE_THRESHOLD = 60;
const HIT_SLOP = minTouchTarget / 2;

type AdvanceControlProps = {
  onAdvance: () => void;
  accessibilityLabel?: string;
  children?: ReactNode;
};

/**
 * Swipe-left plus chevron-forward advance, both wired to the same
 * callback, so a screen cannot ship the swipe without the visible
 * control (a swipe-only advance is undiscoverable and not accessible by
 * default). A rightward release does nothing: UI-SPEC assigns backward
 * movement to the platform's own back gesture, so intercepting it here
 * would produce two competing back paths.
 */
export function AdvanceControl({ onAdvance, accessibilityLabel = 'Next', children }: AdvanceControlProps) {
  const panResponder = useRef(
    PanResponder.create({
      onMoveShouldSetPanResponder: (
        _event: GestureResponderEvent,
        gestureState: PanResponderGestureState
      ) => {
        const { dx, dy } = gestureState;
        return Math.abs(dx) > Math.abs(dy) * 2 && Math.abs(dx) > 24;
      },
      onPanResponderRelease: (_event: GestureResponderEvent, gestureState: PanResponderGestureState) => {
        if (gestureState.dx < -SWIPE_THRESHOLD) {
          onAdvance();
        }
      },
    })
  ).current;

  return (
    <View style={{ flex: 1 }} {...panResponder.panHandlers}>
      {children}
      <Pressable
        onPress={onAdvance}
        accessibilityRole="button"
        accessibilityLabel={accessibilityLabel}
        hitSlop={HIT_SLOP}
        style={{
          position: 'absolute',
          right: space.lg,
          bottom: space.lg,
        }}
      >
        <Ionicons name="chevron-forward" size={24} color={color.ink} />
      </Pressable>
    </View>
  );
}

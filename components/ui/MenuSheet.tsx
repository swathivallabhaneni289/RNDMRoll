import { useEffect } from 'react';
import { Modal, Pressable, View } from 'react-native';
import Animated, { Easing, useAnimatedStyle, useSharedValue, withTiming } from 'react-native-reanimated';
import { AppText } from '@/components/ui/AppText';
import { useReduceMotion } from '@/lib/motion/useReduceMotion';
import { color, elevation, radius, space } from '@/lib/theme/tokens';

const ROW_MIN_HEIGHT = 56;
const SLIDE_FROM = 320;
const SLIDE_MS = 240;

export type MenuItem = { label: string; onPress: () => void };

/**
 * A bottom menu that slides up from the screen edge over a dimmed page, rows in the page's ink
 * colour with hairlines between them, and nothing red. A tap on the dim area or the system back
 * gesture closes it. Under Reduce Motion it appears at once.
 */
export function MenuSheet({
  visible,
  items,
  onClose,
}: {
  visible: boolean;
  items: MenuItem[];
  onClose: () => void;
}) {
  const reduceMotion = useReduceMotion();
  const offset = useSharedValue(SLIDE_FROM);

  useEffect(() => {
    if (!visible) return;
    if (reduceMotion) {
      offset.value = 0;
      return;
    }
    offset.value = SLIDE_FROM;
    offset.value = withTiming(0, { duration: SLIDE_MS, easing: Easing.out(Easing.cubic) });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [visible]);

  const animated = useAnimatedStyle(() => ({ transform: [{ translateY: offset.value }] }));

  return (
    <Modal visible={visible} transparent animationType="none" onRequestClose={onClose}>
      <View style={{ flex: 1, backgroundColor: color.scrimOverlay, justifyContent: 'flex-end' }}>
        <Pressable onPress={onClose} accessibilityRole="button" accessibilityLabel="Close menu" style={{ flex: 1 }} />
        <Animated.View
          accessibilityViewIsModal
          style={[
            {
              ...elevation.card,
              borderTopLeftRadius: radius.lg,
              borderTopRightRadius: radius.lg,
              paddingHorizontal: space.lg,
              paddingTop: space.sm,
              paddingBottom: space.xl,
            },
            animated,
          ]}
        >
          {items.map((item, index) => (
            <Pressable
              key={item.label}
              onPress={item.onPress}
              accessibilityRole="button"
              style={{
                minHeight: ROW_MIN_HEIGHT,
                justifyContent: 'center',
                borderTopWidth: index === 0 ? 0 : 1,
                borderTopColor: color.divider,
              }}
            >
              <AppText role="button">{item.label}</AppText>
            </Pressable>
          ))}
        </Animated.View>
      </View>
    </Modal>
  );
}

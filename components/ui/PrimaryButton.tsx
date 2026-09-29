import { ActivityIndicator, GestureResponderEvent, Pressable, Text } from 'react-native';
import Animated, { Easing, useAnimatedStyle, useSharedValue, withSpring, withTiming } from 'react-native-reanimated';
import { AppText } from '@/components/ui/AppText';
import { color, elevation, minTouchTarget, radius, type } from '@/lib/theme/tokens';

type PrimaryButtonProps = {
  label: string;
  onPress?: (event: GestureResponderEvent) => void;
  disabled?: boolean;
  loading?: boolean;
  variant?: 'standard' | 'onPhoto';
};

/**
 * The raised-elevation Ink-fill primary CTA. `variant="onPhoto"` is UI-SPEC
 * revision 9's single-use inversion for the Welcome cover screen's
 * "Get started" button: same shadow spec and `md` radius cap, fill and
 * label colors inverted so the button separates from the dark Ink scrim
 * behind it rather than merging into it. Both variants stay strictly
 * inside the existing Ink/Dominant pair.
 */
export function PrimaryButton({
  label,
  onPress,
  disabled = false,
  loading = false,
  variant = 'standard',
}: PrimaryButtonProps) {
  const isInert = disabled || loading;
  const isOnPhoto = variant === 'onPhoto';

  const containerStyle = isInert
    ? { ...elevation.subtle, backgroundColor: color.inkDisabled }
    : isOnPhoto
      ? { ...elevation.raised, backgroundColor: color.dominant }
      : { ...elevation.raised };

  // docs/motion-interaction-direction.md Section 10: "subtle press feedback
  // on interactive elements", built once here rather than per screen since
  // every PrimaryButton call site gets it for free. Skipped entirely while
  // inert so a disabled/loading button never looks tappable.
  const scale = useSharedValue(1);
  const pressStyle = useAnimatedStyle(() => ({ transform: [{ scale: scale.value }] }));

  return (
    <Animated.View style={isInert ? undefined : pressStyle}>
      <Pressable
        onPress={isInert ? undefined : onPress}
        onPressIn={
          isInert
            ? undefined
            : () => {
                scale.value = withTiming(0.97, { duration: 90, easing: Easing.out(Easing.cubic) });
              }
        }
        onPressOut={
          isInert
            ? undefined
            : () => {
                scale.value = withSpring(1, { damping: 16, stiffness: 220, overshootClamping: true });
              }
        }
        accessibilityRole="button"
        accessibilityState={{ disabled, busy: loading }}
        style={{
          ...containerStyle,
          borderRadius: radius.md,
          minHeight: minTouchTarget,
          alignItems: 'center',
          justifyContent: 'center',
          paddingHorizontal: 16,
        }}
      >
        {loading ? (
          <ActivityIndicator color={isOnPhoto ? color.ink : color.dominant} />
        ) : disabled ? (
          <Text
            style={{
              fontSize: type.button.fontSize,
              lineHeight: type.button.lineHeight,
              fontFamily: type.button.fontFamily,
              color: color.onInkDisabled,
            }}
          >
            {label}
          </Text>
        ) : (
          <AppText role="button" tone={isOnPhoto ? 'default' : 'onInk'}>
            {label}
          </AppText>
        )}
      </Pressable>
    </Animated.View>
  );
}

import { ActivityIndicator, GestureResponderEvent, Pressable, Text } from 'react-native';
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

  return (
    <Pressable
      onPress={isInert ? undefined : onPress}
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
  );
}

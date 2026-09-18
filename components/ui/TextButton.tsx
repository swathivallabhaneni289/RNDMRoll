import { GestureResponderEvent, Pressable } from 'react-native';
import { AppText } from '@/components/ui/AppText';
import { minTouchTarget, radius } from '@/lib/theme/tokens';

type TextButtonProps = {
  label: string;
  onPress?: (event: GestureResponderEvent) => void;
  disabled?: boolean;
  tone?: 'default' | 'muted' | 'destructive' | 'success' | 'onInk';
};

const HIT_SLOP = minTouchTarget / 2;

/**
 * The "Skip for now" / "Resend email" treatment: plain text, no fill,
 * no border, no shadow. Reaches the minimum tap-target size via
 * `hitSlop` rather than a visible container.
 */
export function TextButton({ label, onPress, disabled = false, tone = 'default' }: TextButtonProps) {
  return (
    <Pressable
      onPress={disabled ? undefined : onPress}
      accessibilityRole="button"
      accessibilityState={{ disabled }}
      hitSlop={HIT_SLOP}
      style={{ borderRadius: radius.md }}
    >
      <AppText role="button" tone={disabled ? 'muted' : tone}>
        {label}
      </AppText>
    </Pressable>
  );
}

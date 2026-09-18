import { ActivityIndicator, GestureResponderEvent, Pressable, View } from 'react-native';
import { Ionicons } from '@expo/vector-icons';
import { AppText } from '@/components/ui/AppText';
import { color, minTouchTarget, radius, space } from '@/lib/theme/tokens';

type Provider = 'email' | 'apple' | 'google';

const GLYPH_BY_PROVIDER: Record<Provider, React.ComponentProps<typeof Ionicons>['name']> = {
  email: 'mail-outline',
  apple: 'logo-apple',
  google: 'logo-google',
};

type MethodButtonProps = {
  provider: Provider;
  label: string;
  onPress?: (event: GestureResponderEvent) => void;
  loading?: boolean;
  disabled?: boolean;
};

/**
 * Full-width social/email sign-in button. UI-SPEC's social sign-in
 * contract requires the screen to disable all three buttons while any
 * one round-trip is in flight; this component only renders the state
 * it is told via `loading` / `disabled`.
 */
export function MethodButton({ provider, label, onPress, loading = false, disabled = false }: MethodButtonProps) {
  const isInert = disabled || loading;

  return (
    <Pressable
      onPress={isInert ? undefined : onPress}
      accessibilityRole="button"
      accessibilityState={{ disabled, busy: loading }}
      style={{
        flexDirection: 'row',
        alignItems: 'center',
        justifyContent: 'center',
        width: '100%',
        minHeight: minTouchTarget,
        backgroundColor: color.secondary,
        borderRadius: radius.md,
        borderWidth: 1,
        borderColor: color.inkRest,
      }}
    >
      {loading ? (
        <ActivityIndicator color={color.ink} />
      ) : (
        <View style={{ flexDirection: 'row', alignItems: 'center' }}>
          <Ionicons
            name={GLYPH_BY_PROVIDER[provider]}
            size={20}
            color={color.ink}
            style={{ marginRight: space.sm }}
          />
          <AppText role="button">{label}</AppText>
        </View>
      )}
    </Pressable>
  );
}

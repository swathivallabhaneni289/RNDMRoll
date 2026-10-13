import { Pressable, View } from 'react-native';
import { Ionicons } from '@expo/vector-icons';
import { AppText } from '@/components/ui/AppText';
import { color, space } from '@/lib/theme/tokens';

/** Every row of Settings is at least this tall (02.1-UI-SPEC section 4). */
export const SETTINGS_ROW_MIN_HEIGHT = 56;

type SettingsRowProps = {
  label: string;
  /** A value at the right, one line, cut with an ellipsis (for example the @username). */
  value?: string;
  onPress: () => void;
  /** `destructive` is the red Log out text. */
  tone?: 'default' | 'destructive';
  /** The trailing chevron; on by default, off for an action that opens a sheet. */
  chevron?: boolean;
  accessibilityHint?: string;
  /** What is spoken; the label (and the value, when there is one) when this is left out. */
  accessibilityLabel?: string;
};

/**
 * One plain row of the Settings list: Body label at the left, an optional muted value and a
 * chevron at the right, a hairline under it, a `secondary` fill while pressed. No radius.
 */
export function SettingsRow({
  label,
  value,
  onPress,
  tone = 'default',
  chevron = true,
  accessibilityHint,
  accessibilityLabel,
}: SettingsRowProps) {
  return (
    <Pressable
      onPress={onPress}
      accessibilityRole="button"
      accessibilityLabel={accessibilityLabel ?? (value ? `${label}, ${value}` : label)}
      accessibilityHint={accessibilityHint}
      style={({ pressed }) => ({
        minHeight: SETTINGS_ROW_MIN_HEIGHT,
        flexDirection: 'row',
        alignItems: 'center',
        borderBottomWidth: 1,
        borderBottomColor: color.divider,
        backgroundColor: pressed ? color.secondary : 'transparent',
      })}
    >
      <AppText role="body" tone={tone === 'destructive' ? 'destructive' : 'default'}>
        {label}
      </AppText>
      <View style={{ flex: 1, alignItems: 'flex-end', marginLeft: space.md }}>
        {value ? (
          <AppText role="body" tone="muted" numberOfLines={1} ellipsizeMode="tail">
            {value}
          </AppText>
        ) : null}
      </View>
      {chevron ? (
        <View style={{ marginLeft: space.xs }}>
          <Ionicons name="chevron-forward" size={20} color={color.muted} />
        </View>
      ) : null}
    </Pressable>
  );
}

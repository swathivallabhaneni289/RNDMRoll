import { useState } from 'react';
import { ActivityIndicator, Text, TextInput, TextInputProps, View } from 'react-native';
import { Ionicons } from '@expo/vector-icons';
import { AppText } from '@/components/ui/AppText';
import { color, minTouchTarget, radius, space, type } from '@/lib/theme/tokens';

type FieldStatus = 'idle' | 'checking' | 'available' | 'taken';

const MULTILINE_PADDING = 12;

type TextFieldProps = {
  label: string;
  error?: string;
  status?: FieldStatus;
  /** Shows "n / max" under the field, right-aligned. Needs `maxLength`. */
  showCount?: boolean;
} & Omit<TextInputProps, 'style'>;

/**
 * Labeled text input with the UI-SPEC rest/focus border step (1dp
 * `color.inkRest` at rest, 1.5dp `color.ink` when focused) and the
 * destructive error slot below the field. The optional `status` prop
 * drives the inline trailing adornment used only by the username step;
 * `taken` renders nothing here because the taken state uses the error
 * slot instead.
 *
 * Deviation: the plan calls for `accessibilityInvalid` on the error
 * state, but that prop does not exist on React Native's TextInput type
 * (verified against the installed RN types: no `accessibilityInvalid`
 * anywhere in ViewAccessibility.d.ts or TextInput.d.ts). Using
 * `accessibilityLiveRegion="polite"` on the error text instead, which
 * announces the error to screen readers on appearance, the same
 * functional goal via a prop that actually typechecks.
 */
export function TextField({
  label,
  error,
  status = 'idle',
  showCount = false,
  onFocus,
  onBlur,
  ...rest
}: TextFieldProps) {
  const [focused, setFocused] = useState(false);
  const multiline = rest.multiline === true;
  const lines = rest.numberOfLines ?? 3;
  const count = typeof rest.value === 'string' ? rest.value.length : 0;

  return (
    <View>
      <AppText role="label">{label}</AppText>
      <TextInput
        {...rest}
        onFocus={(event) => {
          setFocused(true);
          onFocus?.(event);
        }}
        onBlur={(event) => {
          setFocused(false);
          onBlur?.(event);
        }}
        style={{
          backgroundColor: color.secondary,
          borderRadius: radius.md,
          borderWidth: focused ? 1.5 : 1,
          borderColor: focused ? color.ink : color.inkRest,
          // iOS ignores numberOfLines for height, so a multiline field takes its height from the line count here.
          minHeight: multiline ? type.body.lineHeight * lines + MULTILINE_PADDING * 2 : minTouchTarget,
          paddingHorizontal: 12,
          ...(multiline
            ? { paddingTop: MULTILINE_PADDING, paddingBottom: MULTILINE_PADDING, textAlignVertical: 'top' as const }
            : null),
          fontSize: type.body.fontSize,
          lineHeight: type.body.lineHeight,
          fontFamily: type.body.fontFamily,
          color: color.ink,
        }}
      />
      {status === 'checking' ? (
        <View
          accessible
          accessibilityLabel="Checking availability"
          style={{ flexDirection: 'row', alignItems: 'center', marginTop: 4 }}
        >
          <ActivityIndicator size="small" color={color.ink} />
          <AppText role="label" tone="muted">
            {' Checking...'}
          </AppText>
        </View>
      ) : null}
      {status === 'available' ? (
        <View
          accessible
          accessibilityLabel="Username available"
          style={{ flexDirection: 'row', alignItems: 'center', marginTop: 4 }}
        >
          <Ionicons name="checkmark-circle" size={16} color={color.success} />
          <AppText role="label" tone="success">
            {' Available'}
          </AppText>
        </View>
      ) : null}
      {error ? (
        <Text
          accessibilityLiveRegion="polite"
          style={{
            fontSize: type.label.fontSize,
            lineHeight: type.label.lineHeight,
            fontFamily: type.label.fontFamily,
            color: color.destructive,
            textAlign: 'left',
            marginTop: 4,
          }}
        >
          {error}
        </Text>
      ) : null}
      {showCount && rest.maxLength ? (
        <View style={{ marginTop: space.xs, alignItems: 'flex-end' }}>
          <AppText role="label" tone="muted" accessibilityLabel={`${count} of ${rest.maxLength} characters`}>
            {`${count} / ${rest.maxLength}`}
          </AppText>
        </View>
      ) : null}
    </View>
  );
}

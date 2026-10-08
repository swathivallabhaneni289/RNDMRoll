import { useState } from 'react';
import { ActivityIndicator, Pressable, Text, TextInput, TextInputProps, View } from 'react-native';
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
  /** `line` is a flat field with only a bottom rule. `soft` is the profile form look: a small caps label above a light, thin-bordered box, with the count inside it. */
  variant?: 'box' | 'line' | 'soft';
  /** Text shown inside the box before the input (the "@" of a username). `soft` only. */
  prefix?: string;
  /** A small clear button inside the box, shown while there is text. `soft` only. */
  clearable?: boolean;
  /** A short note on the label row, right-aligned, for example Optional. `soft` only. */
  hint?: string;
  /** A short green note under the field when its value is good, for example "Looks good.". Hidden while an error shows. */
  success?: string;
} & Omit<TextInputProps, 'style'>;

/** The green check and note shown under a field whose value is good (the look of the username's "Available"). */
export function FieldSuccess({ text, label }: { text: string; label: string }) {
  return (
    <View
      accessible
      accessibilityLabel={`${label}: ${text}`}
      style={{ flexDirection: 'row', alignItems: 'center', marginTop: 4 }}
    >
      <Ionicons name="checkmark-circle" size={16} color={color.success} />
      <AppText role="label" tone="success">
        {` ${text}`}
      </AppText>
    </View>
  );
}

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
  variant = 'box',
  prefix,
  clearable = false,
  hint,
  success,
  onFocus,
  onBlur,
  ...rest
}: TextFieldProps) {
  const [focused, setFocused] = useState(false);
  const line = variant === 'line';
  const soft = variant === 'soft';
  const multiline = rest.multiline === true;
  const lines = rest.numberOfLines ?? 3;
  const count = typeof rest.value === 'string' ? rest.value.length : 0;
  const counting = showCount && Boolean(rest.maxLength);

  const handleFocus: TextInputProps['onFocus'] = (event) => {
    setFocused(true);
    onFocus?.(event);
  };
  const handleBlur: TextInputProps['onBlur'] = (event) => {
    setFocused(false);
    onBlur?.(event);
  };

  const feedback = (
    <>
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
      {success && !error && status === 'idle' ? <FieldSuccess text={success} label={label} /> : null}
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
    </>
  );

  if (soft) {
    return (
      <View>
        <View style={{ flexDirection: 'row', alignItems: 'baseline', justifyContent: 'space-between' }}>
          <Text
            style={{
              fontFamily: type.button.fontFamily,
              fontSize: type.label.fontSize,
              lineHeight: type.label.lineHeight,
              letterSpacing: 2,
              textTransform: 'uppercase',
              color: color.muted,
            }}
          >
            {label}
          </Text>
          {hint ? (
            <Text
              style={{
                fontFamily: type.label.fontFamily,
                fontSize: type.label.fontSize,
                lineHeight: type.label.lineHeight,
                color: color.muted,
              }}
            >
              {hint}
            </Text>
          ) : null}
        </View>
        <View
          style={{
            marginTop: space.xs,
            backgroundColor: color.fieldSoft,
            borderRadius: radius.md,
            borderWidth: focused ? 1.5 : 1,
            borderColor: focused ? color.ink : color.inkRest,
            paddingHorizontal: 14,
            minHeight: multiline ? type.body.lineHeight * lines + MULTILINE_PADDING * 2 + (counting ? 14 : 0) : minTouchTarget + 6,
            flexDirection: 'row',
            alignItems: multiline ? 'flex-start' : 'center',
            paddingTop: multiline ? MULTILINE_PADDING : 0,
            paddingBottom: multiline ? MULTILINE_PADDING + (counting ? 14 : 0) : 0,
          }}
        >
          {prefix ? (
            <Text
              style={{
                fontSize: type.body.fontSize,
                lineHeight: type.body.lineHeight,
                fontFamily: type.body.fontFamily,
                color: color.muted,
                marginTop: 5,
              }}
            >
              {prefix}
            </Text>
          ) : null}
          <TextInput
            {...rest}
            onFocus={handleFocus}
            onBlur={handleBlur}
            style={{
              flex: 1,
              padding: 0,
              fontSize: type.body.fontSize,
              lineHeight: type.body.lineHeight,
              fontFamily: type.body.fontFamily,
              color: color.ink,
              textAlignVertical: multiline ? 'top' : 'center',
              paddingRight: clearable ? 28 : 0,
            }}
          />
          {clearable && count > 0 ? (
            <Pressable
              onPress={() => rest.onChangeText?.('')}
              accessibilityRole="button"
              accessibilityLabel={`Clear ${label.toLowerCase()}`}
              hitSlop={10}
              style={{ position: 'absolute', top: multiline ? 10 : undefined, right: 10 }}
            >
              <Ionicons name="close-circle" size={20} color={color.inkAvatarPlaceholder} />
            </Pressable>
          ) : null}
          {counting ? (
            <Text
              accessibilityLabel={`${count} of ${rest.maxLength} characters`}
              style={{
                position: 'absolute',
                right: 14,
                bottom: 10,
                fontSize: type.label.fontSize,
                lineHeight: type.label.lineHeight,
                fontFamily: type.label.fontFamily,
                color: color.muted,
              }}
            >
              {`${count}/${rest.maxLength}`}
            </Text>
          ) : null}
        </View>
        {feedback}
      </View>
    );
  }

  return (
    <View>
      <AppText role="label">{label}</AppText>
      <TextInput
        {...rest}
        onFocus={handleFocus}
        onBlur={handleBlur}
        style={{
          backgroundColor: line ? 'transparent' : color.secondary,
          borderRadius: line ? 0 : radius.md,
          borderWidth: line ? 0 : focused ? 1.5 : 1,
          borderBottomWidth: focused ? 1.5 : 1,
          borderColor: focused ? color.ink : color.inkRest,
          // iOS ignores numberOfLines for height, so a multiline field takes its height from the line count here.
          minHeight: multiline ? type.body.lineHeight * lines + MULTILINE_PADDING * 2 : minTouchTarget,
          paddingHorizontal: line ? 0 : 12,
          ...(multiline
            ? { paddingTop: MULTILINE_PADDING, paddingBottom: MULTILINE_PADDING, textAlignVertical: 'top' as const }
            : null),
          fontSize: type.body.fontSize,
          lineHeight: type.body.lineHeight,
          fontFamily: type.body.fontFamily,
          color: color.ink,
        }}
      />
      {feedback}
      {counting ? (
        <View style={{ marginTop: space.xs, alignItems: 'flex-end' }}>
          <AppText role="label" tone="muted" accessibilityLabel={`${count} of ${rest.maxLength} characters`}>
            {`${count} / ${rest.maxLength}`}
          </AppText>
        </View>
      ) : null}
    </View>
  );
}

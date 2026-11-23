import { useState, type Ref } from 'react';
import { ActivityIndicator, Pressable, Text, TextInput, TextInputProps, useWindowDimensions, View } from 'react-native';
import { Ionicons } from '@expo/vector-icons';
import { AppText } from '@/components/ui/AppText';
import { FIELD_VIEW_ID } from '@/components/ui/useKeepFocusedInputInView';
import { color, minTouchTarget, radius, space, type } from '@/lib/theme/tokens';

type FieldStatus = 'idle' | 'checking' | 'available' | 'taken';

const MULTILINE_PADDING = 12;
// A single-line input gets no lineHeight and a frame taller than its text. iOS clips a focused
// single-line field to its frame, and with lineHeight set the text sat low in a 24 high frame, so
// the underscore and the tails of g, j, p, q and y were cut off while typing.
const SINGLE_LINE_INPUT_HEIGHT = 40;

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
  /**
   * A password box: dots until the eye button at the box's right end is tapped, and a second tap
   * hides it again (developer, 2026-11-23: an eye in the box, as most apps have). Its text never
   * gets capitals or autocorrect. Not with `clearable`.
   */
  revealable?: boolean;
  /** A short note on the label row, right-aligned, for example Optional. `soft` only. */
  hint?: string;
  /** A short green note under the field when its value is good, for example "Looks good.". Hidden while an error shows. */
  success?: string;
  /** The TextInput, so a form can call focus() on a box with a problem. */
  ref?: Ref<TextInput>;
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

// AppText's cap on Larger Text. The alert mark follows the first line of the words up to it.
const LARGER_TEXT_CAP = 1.3;

/**
 * A field's problem under the field: an ink alert mark, then the words in ink at Label size. Grey
 * words under a field are a hint and a green tick is "Looks good.", so the mark is what tells a
 * problem apart (no red: red is only for Log out). The mark sits on the first line when the words
 * take two. VoiceOver skips the mark and reads the words once, after the field's label when one is
 * given.
 */
export function FieldError({ text, label }: { text: string; label?: string }) {
  const { fontScale } = useWindowDimensions();
  return (
    <View
      accessible
      accessibilityLabel={label ? `${label}: ${text}` : text}
      // Android only in this React Native. On iPhone the box's hint and announceProblem say it.
      accessibilityLiveRegion="polite"
      style={{ flexDirection: 'row', alignItems: 'flex-start', marginTop: space.xs }}
    >
      <View
        accessibilityElementsHidden
        importantForAccessibility="no-hide-descendants"
        style={{ height: type.label.lineHeight * Math.min(fontScale, LARGER_TEXT_CAP), justifyContent: 'center' }}
      >
        <Ionicons name="alert-circle" size={16} color={color.ink} />
      </View>
      {/* Shrinks to wrap the words, and stays as wide as they are in a centred column. */}
      <View style={{ flexShrink: 1, marginLeft: space.xs }}>
        <AppText role="label">{text}</AppText>
      </View>
    </View>
  );
}

/**
 * Labeled text input with the UI-SPEC rest/focus border step (1dp
 * `color.inkRest` at rest, 1.5dp `color.ink` when focused or while an
 * error shows) and the message slot below the field: the green
 * FieldSuccess note, or the FieldError that replaces it. The optional
 * `status` prop drives the inline trailing adornment used only by the
 * username step; `taken` renders nothing here because the taken state
 * uses the error slot instead.
 *
 * VoiceOver: React Native's TextInput has no `accessibilityInvalid`, and
 * `accessibilityLiveRegion` is Android only, so nothing is said when an
 * error appears. The error is the box's accessibilityHint instead, read
 * when the box gets focus, and a form that shows its errors on a tap of
 * its grey button says the first one with announceProblem
 * (lib/forms/announce.ts). `ref` reaches the TextInput (React 19 passes
 * it as a prop), so a form can call focus() on a box with a problem.
 * The field's own view carries FIELD_VIEW_ID, so when the keyboard comes
 * up the keyboard hook keeps the label, the box and the line under it in
 * view together (useKeepFocusedInputInView).
 */
export function TextField({
  label,
  error,
  status = 'idle',
  showCount = false,
  variant = 'box',
  prefix,
  clearable = false,
  revealable = false,
  hint,
  success,
  onFocus,
  onBlur,
  ref,
  ...rest
}: TextFieldProps) {
  const [focused, setFocused] = useState(false);
  const [shown, setShown] = useState(false);
  // An error keeps the focused border, so the box is marked as well as the words under it.
  const marked = focused || Boolean(error);
  const line = variant === 'line';
  const soft = variant === 'soft';
  const multiline = rest.multiline === true;
  const lines = rest.numberOfLines ?? 3;
  const count = typeof rest.value === 'string' ? rest.value.length : 0;
  const counting = showCount && Boolean(rest.maxLength);
  // A shown password must stay as typed, so a password box turns off capitals and autocorrect.
  const inputProps: TextInputProps = revealable ? { autoCapitalize: 'none', autoCorrect: false, ...rest, secureTextEntry: !shown } : rest;

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
      {error ? <FieldError text={error} label={label} /> : null}
    </>
  );

  // The eye: open while the password is dots, crossed out while it shows.
  const eye = revealable ? (
    <Pressable
      onPress={() => setShown((current) => !current)}
      accessibilityRole="button"
      accessibilityLabel={`${shown ? 'Hide' : 'Show'} ${label.toLowerCase()}`}
      hitSlop={soft ? 12 : undefined}
      style={
        soft
          ? { marginLeft: space.sm }
          : { position: 'absolute', top: 0, right: 0, bottom: 0, width: minTouchTarget, alignItems: 'center', justifyContent: 'center' }
      }
    >
      <Ionicons name={shown ? 'eye-off-outline' : 'eye-outline'} size={22} color={color.muted} />
    </Pressable>
  ) : null;

  if (soft) {
    return (
      <View nativeID={FIELD_VIEW_ID}>
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
            borderWidth: marked ? 1.5 : 1,
            borderColor: marked ? color.ink : color.inkRest,
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
              }}
            >
              {prefix}
            </Text>
          ) : null}
          <TextInput
            {...inputProps}
            ref={ref}
            onFocus={handleFocus}
            onBlur={handleBlur}
            accessibilityHint={error ? error : rest.accessibilityHint}
            style={{
              flex: 1,
              padding: 0,
              fontSize: type.body.fontSize,
              fontFamily: type.body.fontFamily,
              color: color.ink,
              textAlignVertical: multiline ? 'top' : 'center',
              paddingRight: clearable ? 28 : 0,
              ...(multiline ? { lineHeight: type.body.lineHeight } : { height: SINGLE_LINE_INPUT_HEIGHT }),
            }}
          />
          {eye}
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
    <View nativeID={FIELD_VIEW_ID}>
      <AppText role="label">{label}</AppText>
      {/* Its own view, so the eye sits over the box's right end. */}
      <View>
        <TextInput
          {...inputProps}
          ref={ref}
          onFocus={handleFocus}
          onBlur={handleBlur}
          accessibilityHint={error ? error : rest.accessibilityHint}
          style={{
            backgroundColor: line ? 'transparent' : color.secondary,
            borderRadius: line ? 0 : radius.md,
            borderWidth: line ? 0 : marked ? 1.5 : 1,
            borderBottomWidth: marked ? 1.5 : 1,
            borderColor: marked ? color.ink : color.inkRest,
            // iOS ignores numberOfLines for height, so a multiline field takes its height from the line count here.
            minHeight: multiline ? type.body.lineHeight * lines + MULTILINE_PADDING * 2 : minTouchTarget,
            paddingHorizontal: line ? 0 : 12,
            // Room for the eye, so the text never runs under it.
            ...(revealable ? { paddingRight: minTouchTarget } : null),
            ...(multiline
              ? { paddingTop: MULTILINE_PADDING, paddingBottom: MULTILINE_PADDING, textAlignVertical: 'top' as const }
              : null),
            fontSize: type.body.fontSize,
            fontFamily: type.body.fontFamily,
            color: color.ink,
            ...(multiline ? { lineHeight: type.body.lineHeight } : null),
          }}
        />
        {eye}
      </View>
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

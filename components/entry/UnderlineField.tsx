import { useState } from 'react';
import { TextInput, View, type TextInputProps } from 'react-native';
import { CapsLabel } from '@/components/today/CapsLabel';
import { AppText } from '@/components/ui/AppText';
import { color, minTouchTarget, space, type } from '@/lib/theme/tokens';

type UnderlineFieldProps = {
  /** The caps Label above the field. Hidden from screen readers: the field has its own label. */
  label: string;
  value: string;
  onChangeText: (text: string) => void;
  maxLength: number;
  accessibilityLabel: string;
  placeholder?: string;
  multiline?: boolean;
  /** A multiline field grows to this many lines, then scrolls. */
  maxLines?: number;
  /** The count on the label row: only while the field has focus, or always. Default: none. */
  count?: 'focused' | 'always';
  /** Between the number and the limit: '/' gives 12/80, ' / ' gives 12 / 140. */
  countJoin?: string;
  autoCapitalize?: TextInputProps['autoCapitalize'];
  returnKeyType?: TextInputProps['returnKeyType'];
  onFocus?: () => void;
  onBlur?: () => void;
};

/**
 * One underlined text field (UI spec 2 and 4) for the entry's Title and Comment: no box and no
 * fill, a 1dp underline in `ink` when the field has text or focus and in `divider` when it is
 * empty and idle, the text in the `note` role and an optional placeholder in `muted`. The
 * built-in TextField is not used or changed.
 */
export function UnderlineField({
  label,
  value,
  onChangeText,
  maxLength,
  accessibilityLabel,
  placeholder,
  multiline = false,
  maxLines = 3,
  count,
  countJoin = '/',
  autoCapitalize,
  returnKeyType,
  onFocus,
  onBlur,
}: UnderlineFieldProps) {
  const [focused, setFocused] = useState(false);
  const showCount = count === 'always' || (count === 'focused' && focused);
  const lineHeight = type.note.lineHeight;

  return (
    <View>
      <View style={{ flexDirection: 'row', alignItems: 'baseline', justifyContent: 'space-between' }}>
        <View accessibilityElementsHidden importantForAccessibility="no-hide-descendants">
          <CapsLabel>{label}</CapsLabel>
        </View>
        {showCount ? (
          <AppText
            role="label"
            tone="muted"
            accessibilityLabel={`${value.length} of ${maxLength} characters`}
          >
            {`${value.length}${countJoin}${maxLength}`}
          </AppText>
        ) : null}
      </View>
      <TextInput
        value={value}
        onChangeText={onChangeText}
        onFocus={() => {
          setFocused(true);
          onFocus?.();
        }}
        onBlur={() => {
          setFocused(false);
          onBlur?.();
        }}
        maxLength={maxLength}
        multiline={multiline}
        placeholder={placeholder}
        placeholderTextColor={color.muted}
        autoCapitalize={autoCapitalize}
        returnKeyType={returnKeyType}
        accessibilityLabel={accessibilityLabel}
        style={{
          marginTop: space.sm,
          padding: 0,
          fontFamily: type.note.fontFamily,
          fontSize: type.note.fontSize,
          color: color.ink,
          // A single line gets no lineHeight and a frame taller than its text: iOS clips a focused
          // single-line field to its frame (see TextField). A multiline field grows with its text.
          ...(multiline
            ? {
                lineHeight,
                minHeight: lineHeight + 2 * space.sm,
                maxHeight: lineHeight * maxLines + 2 * space.sm,
                paddingVertical: space.sm,
                textAlignVertical: 'top' as const,
              }
            : { height: minTouchTarget }),
        }}
      />
      <View style={{ height: 1, backgroundColor: value.length > 0 || focused ? color.ink : color.divider }} />
    </View>
  );
}

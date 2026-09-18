import { ReactNode } from 'react';
import { Pressable, Text, TextProps } from 'react-native';
import { color, type } from '@/lib/theme/tokens';

type AppTextProps = {
  role?: 'body' | 'label' | 'button' | 'heading' | 'display';
  tone?: 'default' | 'muted' | 'destructive' | 'success' | 'onInk';
  children?: ReactNode;
} & Omit<TextProps, 'style' | 'role'>;

const toneColor = {
  default: color.ink,
  muted: color.muted,
  destructive: color.destructive,
  success: color.success,
  onInk: color.dominant,
} as const;

/**
 * The single text primitive every screen in this phase renders through.
 * `role` selects the type token (fontSize/lineHeight/fontFamily); `tone`
 * selects the color. Never sets a numeric font weight directly — weight
 * lives entirely in the token's `fontFamily` string (UI-SPEC platform note).
 */
export function AppText({ role = 'body', tone = 'default', children, ...rest }: AppTextProps) {
  const typeStyle = type[role];
  return (
    <Text
      {...rest}
      style={{
        fontSize: typeStyle.fontSize,
        lineHeight: typeStyle.lineHeight,
        fontFamily: typeStyle.fontFamily,
        color: toneColor[tone],
      }}
    >
      {children}
    </Text>
  );
}

/**
 * The underlined-Ink text link treatment reserved for "Forgot password?",
 * "Log in instead" / "Sign up instead".
 */
export function TextLink({ onPress, children }: { onPress?: () => void; children?: ReactNode }) {
  return (
    <Pressable onPress={onPress} accessibilityRole="button">
      <Text
        style={{
          fontSize: type.button.fontSize,
          lineHeight: type.button.lineHeight,
          fontFamily: type.button.fontFamily,
          color: color.ink,
          textDecorationLine: 'underline',
        }}
      >
        {children}
      </Text>
    </Pressable>
  );
}

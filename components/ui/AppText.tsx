import { ReactNode } from 'react';
import { Pressable, Text, TextProps } from 'react-native';
import { color, minTouchTarget, space, type } from '@/lib/theme/tokens';

type AppTextProps = {
  role?:
    | 'body'
    | 'label'
    | 'button'
    | 'heading'
    | 'display'
    | 'title'
    | 'countdown'
    | 'wheelLabel'
    | 'hero'
    | 'heroSmall'
    | 'note';
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
 * selects the color. Never sets a numeric font weight directly: weight
 * lives entirely in the token's `fontFamily` string (UI-SPEC platform note).
 */
export function AppText({ role = 'body', tone = 'default', children, ...rest }: AppTextProps) {
  const typeStyle = type[role];
  return (
    <Text
      // Larger Text is honoured, but capped so a header or a bar label never breaks the layout.
      maxFontSizeMultiplier={1.3}
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

/** Above and below the link's line, so its tap area is 44 points high while the line keeps its height. */
const LINK_SLOP_Y = (minTouchTarget - type.button.lineHeight) / 2;

/**
 * "Log in instead" / "Sign up instead": ink words in the button weight, never underlined (the
 * developer's rule, 2026-10-18: underlined text reads as unfinished).
 */
export function TextLink({ onPress, children }: { onPress?: () => void; children?: ReactNode }) {
  return (
    <Pressable
      onPress={onPress}
      accessibilityRole="button"
      hitSlop={{ top: LINK_SLOP_Y, bottom: LINK_SLOP_Y, left: space.sm, right: space.sm }}
    >
      <Text
        style={{
          fontSize: type.button.fontSize,
          lineHeight: type.button.lineHeight,
          fontFamily: type.button.fontFamily,
          color: color.ink,
        }}
      >
        {children}
      </Text>
    </Pressable>
  );
}

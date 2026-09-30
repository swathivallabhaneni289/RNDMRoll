import { Text, TextProps } from 'react-native';
import { color, type } from '@/lib/theme/tokens';

type Role = keyof typeof type;

type Props = Omit<TextProps, 'role'> & {
  role?: Role;
  tone?: 'ink' | 'muted';
  /** Ink transparency, for quiet lines that sit on a tinted surface. */
  opacity?: number;
  /**
   * Semibold Work Sans (the button token's family) at the role's own size. For a name
   * that leads a caption or a comment inside a body or label line. Not for headings.
   */
  strong?: boolean;
  uppercase?: boolean;
  /** Letter spacing in dp; small caps labels read better slightly open. */
  tracking?: number;
};

/**
 * A thin superset of AppText for the lab: same type tokens and tone colours, plus
 * the few extras the editorial layouts need (uppercase labels, tracking, reduced
 * ink, a semibold name at label or body size). It never sets a font weight itself;
 * weight lives in the token's fontFamily, exactly as AppText does.
 */
export function LabText({ role = 'body', tone = 'ink', opacity, strong, uppercase, tracking, style, ...rest }: Props) {
  const token = type[role];
  return (
    <Text
      {...rest}
      style={[
        {
          fontSize: token.fontSize,
          lineHeight: token.lineHeight,
          fontFamily: strong ? type.button.fontFamily : token.fontFamily,
          color: tone === 'muted' ? color.muted : color.ink,
        },
        opacity !== undefined ? { opacity } : null,
        uppercase ? { textTransform: 'uppercase' } : null,
        tracking !== undefined ? { letterSpacing: tracking } : null,
        style,
      ]}
    />
  );
}

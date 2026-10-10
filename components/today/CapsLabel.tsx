import { ReactNode } from 'react';
import { Text } from 'react-native';
import { color, type } from '@/lib/theme/tokens';

type CapsLabelProps = {
  children?: ReactNode;
  /** `muted` is the default of every small label on Today; `ink` is for figures that must read first. */
  tone?: 'muted' | 'ink';
  align?: 'left' | 'center' | 'right';
  numberOfLines?: number;
};

/**
 * The small-caps Label of Today: Label role (14/20), letter spacing 1.5, capitals by text style
 * only, so the stored text keeps the case the server wrote. Used for the date line, "This week",
 * the slot A labels and the stamp row ("Spun at 8:02 PM").
 */
export function CapsLabel({ children, tone = 'muted', align = 'left', numberOfLines }: CapsLabelProps) {
  return (
    <Text
      numberOfLines={numberOfLines}
      style={{
        fontFamily: type.label.fontFamily,
        fontSize: type.label.fontSize,
        lineHeight: type.label.lineHeight,
        letterSpacing: 1.5,
        textTransform: 'uppercase',
        textAlign: align,
        color: tone === 'ink' ? color.ink : color.muted,
      }}
    >
      {children}
    </Text>
  );
}

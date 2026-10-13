import { Text } from 'react-native';
import { color, type } from '@/lib/theme/tokens';

/**
 * A small label in capitals by style, wide tracking, `muted`: "YOUR TASTES \u00B7 AVG RATING",
 * "RECENT ACTIVITY" and the "MEAL \u00B7 OCT 8" line of a recent post (04-UI-SPEC 4.3 and 4.4).
 * The text is passed in sentence case and drawn in capitals, so a screen reader reads words.
 * Label role (Work Sans Regular 14/20); `spacing` is the letter spacing (1.5 for a section, 1.2 for
 * the line under a title). `header` marks a section heading for VoiceOver.
 */
export function SectionLabel({
  children,
  spacing = 1.5,
  header = false,
}: {
  children: string;
  spacing?: number;
  header?: boolean;
}) {
  return (
    <Text
      accessibilityRole={header ? 'header' : undefined}
      style={{
        fontFamily: type.label.fontFamily,
        fontSize: type.label.fontSize,
        lineHeight: type.label.lineHeight,
        letterSpacing: spacing,
        textTransform: 'uppercase',
        color: color.muted,
      }}
    >
      {children}
    </Text>
  );
}

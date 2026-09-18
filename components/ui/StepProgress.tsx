import { Text, View } from 'react-native';
import { color, type } from '@/lib/theme/tokens';

/**
 * UI-SPEC revision 9's numeric "NN / 03" step indicator, replacing the
 * retired dot-based indicator (see revision_reason: a dot row is the
 * single most common tell of a generic onboarding carousel).
 *
 * Deviation from the plan's literal wording: the denominator is styled
 * with an explicit `color.muted` / `type.label` reference here rather
 * than delegating through `AppText role="label" tone="muted"`, because
 * this file's own acceptance criteria require the `color.muted` token
 * reference to appear directly in this file. Visual output is identical
 * to what AppText's `muted` tone would produce.
 */
export function StepProgress({ current, total = 3 }: { current: number; total?: number }) {
  const pad = (value: number) => String(value).padStart(2, '0');
  const numerator = `${pad(current)} `;
  const denominator = `/ ${pad(total)}`;

  return (
    <View
      accessibilityRole="progressbar"
      accessibilityValue={{ min: 1, max: total, now: current }}
      accessibilityLabel={`Step ${current} of ${total}`}
      style={{ flexDirection: 'row', alignItems: 'baseline' }}
    >
      <Text
        style={{
          fontSize: type.label.fontSize,
          lineHeight: type.label.lineHeight,
          fontFamily: type.button.fontFamily,
          color: color.ink,
        }}
      >
        {numerator}
      </Text>
      <Text
        style={{
          fontSize: type.label.fontSize,
          lineHeight: type.label.lineHeight,
          fontFamily: type.label.fontFamily,
          color: color.muted,
        }}
      >
        {denominator}
      </Text>
    </View>
  );
}

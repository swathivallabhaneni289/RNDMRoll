import { ReactNode } from 'react';
import { Pressable, Text, View } from 'react-native';
import { CapsLabel } from '@/components/today/CapsLabel';
import { AppText } from '@/components/ui/AppText';
import { FadeInUp } from '@/lib/motion/primitives';
import { color, minTouchTarget, space, type } from '@/lib/theme/tokens';

/** The word is this far below the heading (space.xxxl + space.xl). */
export const RESULT_WORD_TOP = space.xxxl + space.xl;

/** The reveal (02-UI-SPEC section 5): the stamp row follows the word, the link follows the stamp row. */
const STAMP_DELAY_MS = 120;
const LINK_DELAY_MS = 240;

function Step({ animate, delay, children }: { animate: boolean; delay: number; children: ReactNode }) {
  if (!animate) return <View>{children}</View>;
  return <FadeInUp delay={delay}>{children}</FadeInUp>;
}

/**
 * What takes the wheel's place after the spin (02-UI-SPEC 4.3, the developer's picture
 * `today-after-spin-02`): the word the server picked in the `hero` role, left aligned, one
 * line that shrinks to fit; under it a 1dp ink rule with "Spun at 8:02 PM" at the right (the
 * time is `spin.spun_local_time`, printed exactly as the server sent it); and only while a bonus
 * spin is held and the day is not settled, the link "Use bonus spin". No other button here:
 * Phase 3 adds its buttons in `DayActions`, below this block.
 *
 * `animate` runs the reveal (the word rises 14dp, the stamp row 120ms later, the link 120ms
 * after that). Without it everything is simply there, which is how a relaunch or a refetch
 * shows a spin that was already made.
 */
export function ResultBlock({
  name,
  spunLocalTime,
  late = false,
  canUseBonus,
  onUseBonus,
  animate = false,
}: {
  name: string;
  spunLocalTime: string;
  /** The "Last night's spin." heading is showing. */
  late?: boolean;
  canUseBonus: boolean;
  onUseBonus: () => void;
  animate?: boolean;
}) {
  const spoken = `${late ? "Last night's spin" : "Today's spin"}: ${name}.`;
  return (
    <View>
      <Step animate={animate} delay={0}>
        <AppText
          role="hero"
          accessibilityRole="header"
          accessibilityLabel={spoken}
          numberOfLines={1}
          adjustsFontSizeToFit
          minimumFontScale={0.36}
          allowFontScaling={false}
        >
          {name}
        </AppText>
      </Step>

      <View style={{ marginTop: space.lg }}>
        <Step animate={animate} delay={STAMP_DELAY_MS}>
          <View
            accessible
            accessibilityLabel={`Spun at ${spunLocalTime}`}
            style={{ flexDirection: 'row', alignItems: 'center' }}
          >
            <View style={{ flex: 1, height: 1, backgroundColor: color.ink }} />
            <View style={{ width: space.md }} />
            <CapsLabel align="right">{`Spun at ${spunLocalTime}`}</CapsLabel>
          </View>
        </Step>
      </View>

      {canUseBonus ? (
        <View style={{ marginTop: space.md, alignItems: 'center' }}>
          <Step animate={animate} delay={LINK_DELAY_MS}>
            <Pressable
              onPress={onUseBonus}
              accessibilityRole="button"
              hitSlop={8}
              style={{ minHeight: minTouchTarget, justifyContent: 'center', paddingHorizontal: space.md }}
            >
              <Text
                style={{
                  fontFamily: type.button.fontFamily,
                  fontSize: type.button.fontSize,
                  lineHeight: type.button.lineHeight,
                  color: color.ink,
                  textDecorationLine: 'underline',
                }}
              >
                Use bonus spin
              </Text>
            </Pressable>
          </Step>
        </View>
      ) : null}
    </View>
  );
}

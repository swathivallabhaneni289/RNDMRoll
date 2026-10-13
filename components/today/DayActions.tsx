import { ReactNode } from 'react';
import { Text, View } from 'react-native';
import { useRouter } from 'expo-router';
import { PrimaryButton } from '@/components/ui/PrimaryButton';
import { FadeInUp } from '@/lib/motion/primitives';
import { color, space, type } from '@/lib/theme/tokens';

/** The reveal order (02-UI-SPEC section 5): the word, the stamp row 120ms later, these 120ms after that. */
const ACTIONS_DELAY_MS = 240;

function Step({ animate, children }: { animate: boolean; children: ReactNode }) {
  if (!animate) return <View>{children}</View>;
  return <FadeInUp delay={ACTIONS_DELAY_MS}>{children}</FadeInUp>;
}

/**
 * What sits under the stamp row of Result and Late (03-UI-SPEC section 3.2, the developer's picture
 * `today-after-spin-02`): in Late the line 'Posting it now would count as late.', then the 'Log it'
 * button, which pushes the entry screen. Plan 03-03 adds 'Skip this one' under the button.
 *
 * Spacing: `xxl` between the stamp row and 'Log it'; when the late line, the bonus link or a
 * message already sits there (`followsLink`), `md` after the last of them. The button is inert
 * while a spin is running (`disabled`).
 *
 * `animate` runs the reveal's last step (a rise 240ms after the word); a relaunch or a refetch
 * shows everything at once.
 */
export function DayActions({
  late,
  followsLink,
  disabled,
  animate = false,
}: {
  /** The state is Late: last night's spin, and a post now counts as late. */
  late: boolean;
  /** The bonus link or a message sits between the stamp row and these actions. */
  followsLink: boolean;
  disabled: boolean;
  animate?: boolean;
}) {
  const router = useRouter();

  return (
    <Step animate={animate}>
      {late ? (
        <View style={{ marginTop: space.md }}>
          <Text
            style={{
              fontFamily: type.label.fontFamily,
              fontSize: type.label.fontSize,
              lineHeight: type.label.lineHeight,
              color: color.muted,
              textAlign: 'center',
            }}
          >
            Posting it now would count as late.
          </Text>
        </View>
      ) : null}

      <View style={{ marginTop: late || followsLink ? space.md : space.xxl }}>
        <PrimaryButton label="Log it" onPress={() => router.push('/(app)/today/log')} disabled={disabled} />
      </View>
    </Step>
  );
}

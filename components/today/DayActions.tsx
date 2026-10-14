import { ReactNode, useEffect, useRef, useState } from 'react';
import { Alert, Pressable, Text, View } from 'react-native';
import { useRouter } from 'expo-router';
import { PrimaryButton } from '@/components/ui/PrimaryButton';
import { entryErrorMessage, skipDay } from '@/lib/api/entries';
import type { Today } from '@/lib/api/types';
import { skipBody } from '@/lib/entry/rules';
import { FadeInUp } from '@/lib/motion/primitives';
import { color, space, type } from '@/lib/theme/tokens';
import { applyToday } from '@/lib/wheel/todayStore';
import { refetchToday } from '@/lib/wheel/useToday';

/** The reveal order (02-UI-SPEC section 5): the word, the stamp row 120ms later, these 120ms after that. */
const ACTIONS_DELAY_MS = 240;

function Step({ animate, children }: { animate: boolean; children: ReactNode }) {
  if (!animate) return <View>{children}</View>;
  return <FadeInUp delay={ACTIONS_DELAY_MS}>{children}</FadeInUp>;
}

const labelLine = {
  fontFamily: type.label.fontFamily,
  fontSize: type.label.fontSize,
  lineHeight: type.label.lineHeight,
  textAlign: 'center',
} as const;

/**
 * What sits under the stamp row of Result and Late (03-UI-SPEC section 3.2, the developer's picture
 * `today-after-spin-02`): in Late the line 'Posting it now would count as late.', then the 'Log it'
 * button, which pushes the entry screen, and under it the quiet underlined link 'Skip this one'.
 *
 * Skip asks once with the phone's own alert (Cancel, and a red 'Skip it'), sends the date and
 * spin number of the spin that is on screen, and puts the answer's Today into the store; Today
 * then shows Skipped. A skip is final. A failure shows its line under the link; the four state
 * answers also ask the server for Today again. While a skip runs both controls are inert.
 *
 * Spacing: `xxl` between the stamp row and 'Log it'; when the late line, the bonus link or a
 * message already sits there (`followsLink`), `md` after the last of them. Both controls are
 * inert while a spin is running or in the non-live copy (`disabled`).
 *
 * `animate` runs the reveal's last step (a rise 240ms after the word); a relaunch or a refetch
 * shows everything at once.
 */
export function DayActions({
  day,
  userId,
  late,
  followsLink,
  disabled,
  animate = false,
}: {
  /** The Today that is on screen: its spin, week and goal are what the skip names and says. */
  day: Today;
  userId: string | null;
  /** The state is Late: last night's spin, and a post now counts as late. */
  late: boolean;
  /** The bonus link or a message sits between the stamp row and these actions. */
  followsLink: boolean;
  disabled: boolean;
  animate?: boolean;
}) {
  const router = useRouter();
  const [skipping, setSkipping] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  const busyRef = useRef(false);
  const mountedRef = useRef(true);
  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);

  const spin = day.spin;
  const inert = disabled || skipping;

  async function runSkip() {
    if (busyRef.current || userId === null || spin === null) return;
    busyRef.current = true;
    setSkipping(true);
    setMessage(null);
    const mark = performance.now();
    try {
      const answer = await skipDay(spin.local_date, spin.seq);
      // The answer's Today goes in at once, so Today shows Skipped with no loading step. Today is
      // also asked for again: a read that began before the skip can answer after it.
      applyToday(userId, answer.today, mark);
      void refetchToday(userId, 'manual');
      // This block leaves the screen with the new state; it stays inert until then.
    } catch (err) {
      const failure = entryErrorMessage(err);
      if (failure.terminal) void refetchToday(userId, 'manual');
      busyRef.current = false;
      if (mountedRef.current) {
        setSkipping(false);
        setMessage(failure.text);
      }
    }
  }

  function askSkip() {
    if (inert || busyRef.current || spin === null) return;
    Alert.alert(
      'Skip this one?',
      skipBody(spin.local_date, day.week.start, day.week.posted, day.week.goal),
      [
        { text: 'Cancel', style: 'cancel' },
        { text: 'Skip it', style: 'destructive', onPress: () => void runSkip() },
      ],
      { cancelable: true }
    );
  }

  return (
    <Step animate={animate}>
      {late ? (
        <View style={{ marginTop: space.md }}>
          <Text style={{ ...labelLine, color: color.muted }}>Posting it now would count as late.</Text>
        </View>
      ) : null}

      <View style={{ marginTop: late || followsLink ? space.md : space.xxl }}>
        <PrimaryButton label="Log it" onPress={() => router.push('/(app)/today/log')} disabled={inert} />
      </View>

      <View style={{ marginTop: space.md, alignItems: 'center' }}>
        <Pressable
          onPress={askSkip}
          disabled={inert}
          accessibilityRole="button"
          accessibilityLabel="Skip this one"
          accessibilityHint="Asks first."
          accessibilityState={{ disabled: inert }}
          hitSlop={{ top: 12, bottom: 12, left: 16, right: 16 }}
        >
          <Text
            style={{
              fontFamily: type.label.fontFamily,
              fontSize: type.label.fontSize,
              lineHeight: type.label.lineHeight,
              color: color.muted,
              textDecorationLine: 'underline',
            }}
          >
            Skip this one
          </Text>
        </Pressable>
      </View>

      {message !== null ? (
        <View style={{ marginTop: space.md }} accessibilityLiveRegion="polite">
          <Text style={{ ...labelLine, color: color.destructive }}>{message}</Text>
        </View>
      ) : null}
    </Step>
  );
}

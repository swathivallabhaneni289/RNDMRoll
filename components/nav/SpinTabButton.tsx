import { Pressable, Text, View } from 'react-native';
import { useRouter } from 'expo-router';
import { BrandMark } from '@/components/brand/BrandMark';
import { PressScale } from '@/lib/motion/primitives';
import { useReduceMotion } from '@/lib/motion/useReduceMotion';
import { useSession } from '@/lib/session/store';
import { color, radius, type } from '@/lib/theme/tokens';
import { requestPendingSpin, useTodaySnapshot } from '@/lib/wheel/todayStore';

/** The circle's size, and how far its top edge stands above the bar's top edge. */
const CIRCLE = 64;
const RISE = 12;
/** The bar's own top rule, so the circle's top edge is measured from the rule's outer edge. */
const BAR_BORDER = 1;
const MARK = 30;

/**
 * The middle button of the bottom bar (02-UI-SPEC section 8). A circle of 64dp whose top edge
 * stands 12dp above the bar, with "Spin" under it. It is an action, so it never becomes the
 * selected tab: a press goes to Today and asks it to spin once it has loaded and is still open
 * (`pendingSpin` in the Today store; no timer, no route parameters).
 *
 * Two looks, chosen by the server's state (the one place `radius.full` is used for a tappable):
 *   LOCKED  every state except an open, unspun one: a `secondary` (light grey) fill, no outline,
 *           and the app's own wheel mark in `ink`; its wedges are drawn in `secondary` too, so
 *           they read as cut-outs of the disc. Not dimmed and not disabled; it only says that no
 *           spin can start right now.
 *   OPEN    state `open` and the wheel not turning: solid `ink` fill, the mark inverted.
 * No shadow or elevation in either look, and no animation between them. The spoken hint says
 * what a press does, so the two looks are not told by colour alone.
 *
 * The label sits in the strip under the 56dp bar, where the bottom safe area is. A phone with no
 * such strip (a home-button model) would cut the label off; the button itself still shows.
 */
export function SpinTabButton() {
  const router = useRouter();
  const reduceMotion = useReduceMotion();
  const { user } = useSession();
  const userId = user?.id ?? null;
  const snapshot = useTodaySnapshot(userId);

  const state = snapshot.today?.state ?? null;
  const settled = state === 'posted' || state === 'skipped';
  const open = state === 'open' && !snapshot.spinning;
  const hint =
    state === 'open'
      ? "Spins today's wheel."
      : state === 'waiting' || settled
        ? 'Shows the wheel. It opens at 8:00 PM.'
        : "Shows today's spin.";

  function handlePress() {
    // The request goes first, then the navigation: with unsaved changes in Edit wheel the
    // navigation opens the leave sheet, whose guard clears the request (plan 02-02 decision 4).
    // The other way round the guard would clear a flag that is not set yet.
    if (settled) {
      // Home already shows the posted or skipped day; the Spin button shows the wheel at rest.
      router.navigate('/(app)/today/wheel');
      return;
    }
    if (userId !== null) requestPendingSpin(userId);
    router.navigate('/(app)/today');
  }

  const circle = (
    <View
      style={{
        width: CIRCLE,
        height: CIRCLE,
        borderRadius: radius.full,
        alignItems: 'center',
        justifyContent: 'center',
        backgroundColor: open ? color.ink : color.secondary,
      }}
    >
      <BrandMark
        size={MARK}
        color={open ? color.dominant : color.ink}
        wedgeColor={open ? color.ink : color.secondary}
      />
    </View>
  );

  const content = (
    <View style={{ alignItems: 'center' }}>
      {circle}
      <Text
        allowFontScaling={false}
        style={{
          fontFamily: type.label.fontFamily,
          fontSize: type.label.fontSize,
          lineHeight: type.label.lineHeight,
          color: color.muted,
        }}
      >
        Spin
      </Text>
    </View>
  );

  const shared = {
    onPress: handlePress,
    accessibilityRole: 'button' as const,
    accessibilityLabel: 'Spin',
    accessibilityHint: hint,
    style: { marginTop: -(RISE + BAR_BORDER) },
  };

  return (
    <View style={{ flex: 1, alignItems: 'center' }}>
      {reduceMotion ? (
        <Pressable {...shared}>{content}</Pressable>
      ) : (
        <PressScale {...shared}>{content}</PressScale>
      )}
    </View>
  );
}

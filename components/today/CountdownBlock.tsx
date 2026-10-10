import { useEffect, useState } from 'react';
import { Text, View } from 'react-native';
import { CapsLabel } from '@/components/today/CapsLabel';
import { color, space, type } from '@/lib/theme/tokens';
import { msUntil, parseInstant, spokenMinutes, split } from '@/lib/wheel/countdown';
import type { Anchor } from '@/lib/wheel/todayStore';

/**
 * Each pair of digits sits in its own fixed cell, so the row never changes width. The widest
 * pair in Domine Regular at 40dp ("00") is 53.9dp, so the cell is 56dp (a 48dp cell wrapped the
 * pair onto two lines on the device); the narrowest is 32dp, so the pair is centred in the cell.
 */
const CELL_WIDTH = 56;
const COLON_WIDTH = 12;
/** Printed while there is no anchor (on foreground, until a fresh answer arrives). */
const PLACEHOLDER = '--';

function Cell({ text }: { text: string }) {
  return (
    <View style={{ width: CELL_WIDTH, alignItems: 'center' }}>
      <Text
        allowFontScaling={false}
        numberOfLines={1}
        style={{
          fontFamily: type.countdown.fontFamily,
          fontSize: type.countdown.fontSize,
          lineHeight: type.countdown.lineHeight,
          color: color.ink,
        }}
      >
        {text}
      </Text>
    </View>
  );
}

function Colon() {
  return (
    <View style={{ width: COLON_WIDTH, alignItems: 'center' }}>
      <Text
        allowFontScaling={false}
        numberOfLines={1}
        style={{
          fontFamily: type.countdown.fontFamily,
          fontSize: type.countdown.fontSize,
          lineHeight: type.countdown.lineHeight,
          color: color.ink,
        }}
      >
        :
      </Text>
    </View>
  );
}

/**
 * Slot A and slot B of the Waiting state (02-UI-SPEC 4.2): the label "Opens at 8:00 PM" over the
 * digits "05:04:01". The time left is counted from the server's `now` with a monotonic clock
 * (the anchor), never the phone's clock, so a wrong phone clock changes nothing. The tick lives
 * in this component and re-renders nothing else; the spoken line changes once a minute.
 * `onZero` is called once when the count reaches zero (Today then asks the server again).
 */
export function CountdownBlock({
  anchor,
  nextOpenAt,
  onZero,
}: {
  anchor: Anchor | null;
  nextOpenAt: string;
  onZero?: () => void;
}) {
  const [, setTick] = useState(0);
  const target = parseInstant(nextOpenAt);
  const remaining = anchor && target !== null ? msUntil(nextOpenAt, anchor, performance.now()) : null;

  // Re-render exactly when the displayed second changes.
  useEffect(() => {
    if (remaining === null || remaining <= 0) return undefined;
    const wait = (remaining % 1000) + 8;
    const id = setTimeout(() => setTick((t) => t + 1), wait);
    return () => clearTimeout(id);
  });

  const reached = remaining !== null && remaining <= 0;
  useEffect(() => {
    if (reached) onZero?.();
    // Once per anchor: a fresh answer gives a new anchor object.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [reached, anchor]);

  const parts = remaining === null ? null : split(remaining);
  const spoken =
    remaining === null
      ? 'Opens at 8:00 PM'
      : remaining <= 0
        ? 'Opens at 8:00 PM, now'
        : `Opens at 8:00 PM, in ${spokenMinutes(remaining)}`;

  return (
    <View accessible accessibilityLabel={spoken} style={{ alignItems: 'center' }}>
      <CapsLabel align="center">Opens at 8:00 PM</CapsLabel>
      <View style={{ marginTop: space.xs, flexDirection: 'row', alignItems: 'center' }}>
        <Cell text={parts ? parts.hours : PLACEHOLDER} />
        <Colon />
        <Cell text={parts ? parts.minutes : PLACEHOLDER} />
        <Colon />
        <Cell text={parts ? parts.seconds : PLACEHOLDER} />
      </View>
    </View>
  );
}

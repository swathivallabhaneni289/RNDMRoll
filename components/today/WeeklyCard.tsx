import { Text, View } from 'react-native';
import { Circle, Svg } from 'react-native-svg';
import { CapsLabel } from '@/components/today/CapsLabel';
import type { Week } from '@/lib/api/types';
import { color, space, type } from '@/lib/theme/tokens';

/** The slot every mark is drawn in. */
const MARK_SLOT = 12;
const DOT_DIAMETER = 10;
const RING_STROKE = 1.5;
const SMALL_DOT_DIAMETER = 5;

function Mark({ kind }: { kind: 'posted' | 'today' | 'other' }) {
  const mid = MARK_SLOT / 2;
  return (
    <Svg width={MARK_SLOT} height={MARK_SLOT} viewBox={`0 0 ${MARK_SLOT} ${MARK_SLOT}`}>
      {kind === 'posted' ? (
        <Circle cx={mid} cy={mid} r={DOT_DIAMETER / 2} fill={color.ink} />
      ) : kind === 'today' ? (
        <Circle
          cx={mid}
          cy={mid}
          r={DOT_DIAMETER / 2 - RING_STROKE / 2}
          fill="none"
          stroke={color.ink}
          strokeWidth={RING_STROKE}
        />
      ) : (
        <Circle cx={mid} cy={mid} r={SMALL_DOT_DIAMETER / 2} fill={color.divider} />
      )}
    </Svg>
  );
}

/**
 * The week section (02-UI-SPEC section 6). The file and export keep the name Phase 3 uses; it
 * draws an open section between two rules, not a card, and it is not tappable (nothing in Phase
 * 2 has a place to send it). Everything comes from the server: the letters, the weekday names,
 * the counts and which day is today (`week.today_index`), so there is no date maths here.
 *
 * Marks, by precedence: a posted day is a filled ink dot; today, not posted, is an ink ring;
 * any other day (still to come, or past with nothing) is a small `divider` dot. Phase 3 adds
 * the dash for a skipped day.
 */
export function WeeklyCard({ week, bonusHeld }: { week: Week; bonusHeld: boolean }) {
  const spoken = [
    `${week.posted} of ${week.goal} this week.`,
    bonusHeld ? 'You have a bonus spin.' : null,
    week.days.map((day) => `${day.weekday} ${day.posted ? 'posted' : 'not posted'}`).join(', '),
  ]
    .filter((part): part is string => part !== null)
    .join(' ');

  return (
    <View accessible accessibilityLabel={spoken}>
      <View style={{ height: 1, backgroundColor: color.ink }} />

      <View
        style={{
          marginTop: space.md,
          flexDirection: 'row',
          alignItems: 'baseline',
          justifyContent: 'space-between',
        }}
      >
        <CapsLabel>This week</CapsLabel>
        <View style={{ flexDirection: 'row', alignItems: 'baseline' }}>
          <Text
            style={{
              fontFamily: type.heading.fontFamily,
              fontSize: type.heading.fontSize,
              lineHeight: type.heading.lineHeight,
              color: color.ink,
            }}
          >
            {week.posted}
          </Text>
          <Text
            style={{
              fontFamily: type.label.fontFamily,
              fontSize: type.label.fontSize,
              lineHeight: type.label.lineHeight,
              color: color.ink,
            }}
          >
            {` of ${week.goal} this week`}
          </Text>
        </View>
      </View>

      <View style={{ marginTop: space.sm, flexDirection: 'row' }}>
        {week.days.map((day, index) => {
          const kind = day.posted ? 'posted' : index === week.today_index ? 'today' : 'other';
          return (
            <View key={day.date} style={{ flex: 1, alignItems: 'center' }}>
              <Text
                allowFontScaling={false}
                style={{
                  fontFamily: type.label.fontFamily,
                  fontSize: type.label.fontSize,
                  lineHeight: type.label.lineHeight,
                  color: color.muted,
                }}
              >
                {day.letter}
              </Text>
              <View style={{ marginTop: space.xs }}>
                <Mark kind={kind} />
              </View>
            </View>
          );
        })}
      </View>

      {bonusHeld ? (
        <View style={{ marginTop: space.sm }}>
          <Text
            style={{
              fontFamily: type.label.fontFamily,
              fontSize: type.label.fontSize,
              lineHeight: type.label.lineHeight,
              color: color.muted,
            }}
          >
            You have a bonus spin.
          </Text>
        </View>
      ) : null}

      <View style={{ marginTop: space.md, height: 1, backgroundColor: color.divider }} />
    </View>
  );
}

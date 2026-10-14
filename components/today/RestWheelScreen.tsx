import { useWindowDimensions, View } from 'react-native';
import { useRouter } from 'expo-router';
import { Ionicons } from '@expo/vector-icons';
import { Pressable } from 'react-native';
import { AppText } from '@/components/ui/AppText';
import { Screen } from '@/components/ui/Screen';
import { Wheel } from '@/components/wheel/Wheel';
import { CapsLabel } from '@/components/today/CapsLabel';
import { CountdownBlock } from '@/components/today/CountdownBlock';
import { WeeklyCard } from '@/components/today/WeeklyCard';
import { useSession } from '@/lib/session/store';
import { color, minTouchTarget, space } from '@/lib/theme/tokens';
import { dateLineLong } from '@/lib/wheel/format';
import { useTodaySnapshot } from '@/lib/wheel/todayStore';

const WHEEL_MAX = 232;
const noop = () => {};

/**
 * The page the Spin button opens once the day is posted or skipped: the wheel at rest with the
 * countdown to the next spin. Home keeps the entry; this page never repeats it. It only reads
 * the Today the Home screen already holds, and the wheel cannot be turned.
 */
export function RestWheelScreen() {
  const router = useRouter();
  const { width } = useWindowDimensions();
  const { user } = useSession();
  const { today, anchor } = useTodaySnapshot(user?.id ?? null);
  const size = Math.min(WHEEL_MAX, width - 2 * space.lg);

  return (
    <Screen edges={['top', 'left', 'right']}>
      <View style={{ flexGrow: 1 }}>
        <View
          style={{
            height: minTouchTarget,
            flexDirection: 'row',
            alignItems: 'center',
            justifyContent: 'space-between',
          }}
        >
          <AppText role="heading">RNDMRoll</AppText>
          <Pressable
            onPress={() => router.push('/(app)/today/notifications')}
            accessibilityRole="button"
            accessibilityLabel="Notifications"
            accessibilityHint="Opens your notifications."
            hitSlop={4}
            style={{ width: minTouchTarget, height: minTouchTarget, alignItems: 'flex-end', justifyContent: 'center' }}
          >
            <Ionicons name="notifications-outline" size={24} color={color.ink} />
          </Pressable>
        </View>

        <View style={{ marginTop: space.md, minHeight: 20 }}>
          {today ? <CapsLabel>{dateLineLong(today.local_date)}</CapsLabel> : null}
        </View>
        <View style={{ marginTop: space.sm }}>
          <AppText role="title" accessibilityRole="header">
            Your next spin is coming.
          </AppText>
        </View>

        {today ? (
          <>
            <View style={{ marginTop: space.lg, alignItems: 'center' }}>
              <Wheel
                entries={today.wheel}
                spinId={0}
                winnerIndex={null}
                size={size}
                interactive={false}
                onHubPress={noop}
                showPointer
                onLanded={noop}
                hubHint="Opens at 8:00 PM."
              />
            </View>
            <View style={{ marginTop: space.lg, alignItems: 'center' }}>
              <CountdownBlock anchor={anchor} nextOpenAt={today.next_open_at} />
            </View>
            <View style={{ flexGrow: 1 }} />
            <WeeklyCard week={today.week} bonusHeld={today.bonus.held} />
          </>
        ) : null}
        <View style={{ height: space.xl }} />
      </View>
    </Screen>
  );
}

import { useRouter } from 'expo-router';
import { View } from 'react-native';
import Animated, { FadeIn, FadeInDown } from 'react-native-reanimated';
import { AdvanceControl } from '@/components/ui/AdvanceControl';
import { AppText } from '@/components/ui/AppText';
import { PolaroidCard } from '@/components/ui/PhotoPanel';
import { Screen } from '@/components/ui/Screen';
import { StepProgress } from '@/components/ui/StepProgress';
import { space } from '@/lib/theme/tokens';

export default function RitualScreen() {
  const router = useRouter();

  return (
    <Screen scroll={false}>
      <AdvanceControl onAdvance={() => router.push('/real-photos')}>
        <Animated.View entering={FadeIn.delay(220).duration(400)} style={{ paddingTop: space.sm }}>
          <StepProgress current={1} total={3} />
        </Animated.View>

        <View
          style={{
            flexDirection: 'row',
            gap: space.sm,
            marginTop: space.xl,
            marginBottom: space.xl,
          }}
        >
          <Animated.View entering={FadeInDown.delay(330).duration(450)} style={{ flex: 1, marginTop: space.md }}>
            <PolaroidCard rotation={-6} />
          </Animated.View>
          <Animated.View entering={FadeInDown.delay(440).duration(450)} style={{ flex: 1 }}>
            <PolaroidCard rotation={0} />
          </Animated.View>
          <Animated.View entering={FadeInDown.delay(550).duration(450)} style={{ flex: 1, marginTop: space.md }}>
            <PolaroidCard rotation={6} />
          </Animated.View>
        </View>

        <Animated.View entering={FadeInDown.delay(660).duration(450)}>
          <AppText role="display">{"life's better when it's random."}</AppText>
        </Animated.View>

        <Animated.View entering={FadeIn.delay(770).duration(450)} style={{ marginTop: space.md }}>
          <AppText role="body">
            {"Every night at 8:00 PM, your wheel reveals tonight's category."}
          </AppText>
        </Animated.View>
      </AdvanceControl>
    </Screen>
  );
}

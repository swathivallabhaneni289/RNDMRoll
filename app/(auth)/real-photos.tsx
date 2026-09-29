import { useRouter } from 'expo-router';
import { View } from 'react-native';
import Animated, { FadeIn, FadeInDown } from 'react-native-reanimated';
import { AdvanceControl } from '@/components/ui/AdvanceControl';
import { AppText } from '@/components/ui/AppText';
import { PhotoPanel } from '@/components/ui/PhotoPanel';
import { Screen } from '@/components/ui/Screen';
import { StepProgress } from '@/components/ui/StepProgress';
import { space } from '@/lib/theme/tokens';

export default function RealPhotosScreen() {
  const router = useRouter();

  return (
    <Screen scroll={false}>
      <AdvanceControl onAdvance={() => router.push('/everyone-rolls')}>
        <Animated.View entering={FadeIn.delay(220).duration(400)} style={{ paddingTop: space.sm }}>
          <StepProgress current={2} total={3} />
        </Animated.View>

        <Animated.View
          entering={FadeInDown.delay(330).duration(500)}
          style={{ marginTop: space.xl, marginBottom: space.xl }}
        >
          <PhotoPanel />
        </Animated.View>

        <Animated.View entering={FadeInDown.delay(440).duration(450)}>
          <AppText role="display">{'real photos only.'}</AppText>
        </Animated.View>

        <Animated.View entering={FadeIn.delay(550).duration(450)} style={{ marginTop: space.md }}>
          <AppText role="body">
            {'No posters. No album covers. No stock images. Only moments you actually captured.'}
          </AppText>
        </Animated.View>
      </AdvanceControl>
    </Screen>
  );
}

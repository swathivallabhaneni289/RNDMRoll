import { useRouter } from 'expo-router';
import { useState } from 'react';
import { View } from 'react-native';
import { WHEEL_SECTORS } from '@/components/brand/CategoryRow';
import { SpinningMark } from '@/components/brand/SpinningMark';
import { AppText } from '@/components/ui/AppText';
import { PrimaryButton } from '@/components/ui/PrimaryButton';
import { Screen } from '@/components/ui/Screen';
import { FadeInUp, ImageReveal } from '@/lib/motion/primitives';
import { space } from '@/lib/theme/tokens';

/**
 * The single pre-signup page (UI-SPEC revision 11): the wheel logo spins and stops, then
 * Get started goes straight to the method chooser. The earlier Ritual, Real > Perfect and
 * Everyone Spins pages were removed as not useful.
 */
export default function WelcomeScreen() {
  const router = useRouter();
  const [picked, setPicked] = useState<string | null>(null);
  const [kick, setKick] = useState(0); // bumped whenever the wheel starts moving; the background wheels spin up

  // Each time the wheel comes to rest, the category under its pointer lights up in the background.
  function handleLanded(word: string | null) {
    setPicked(word);
  }

  function handleGetStarted() {
    router.push('/choose-method');
  }

  return (
    <Screen scroll={false} wheels wheelLabel={picked} wheelKick={kick}>
      <FadeInUp delay={60} duration={500} style={{ paddingTop: space.sm }}>
        <AppText role="label" tone="muted">
          {'one spin.\none real moment.'}
        </AppText>
      </FadeInUp>

      <View style={{ flex: 1, justifyContent: 'center', alignItems: 'center' }}>
        <ImageReveal delay={120} duration={600}>
          <SpinningMark size={260} rollIn sectors={WHEEL_SECTORS} onLanded={handleLanded} onSpinStart={() => setKick((k) => k + 1)} />
        </ImageReveal>
        <FadeInUp delay={260} duration={500} style={{ marginTop: space.md }}>
          <AppText role="display">{'RNDMRoll'}</AppText>
        </FadeInUp>
      </View>

      <FadeInUp delay={360} duration={500} style={{ paddingBottom: space.lg }}>
        <PrimaryButton label="Get started" onPress={handleGetStarted} />
      </FadeInUp>
    </Screen>
  );
}

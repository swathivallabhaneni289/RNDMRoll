import { useRouter } from 'expo-router';
import { View } from 'react-native';
import { FadeInUp, ImageReveal } from '@/lib/motion/primitives';
import { AppText } from '@/components/ui/AppText';
import { PrimaryButton } from '@/components/ui/PrimaryButton';
import { SplitPhotoPanels } from '@/components/ui/PhotoPanel';
import { Screen } from '@/components/ui/Screen';
import { StepProgress } from '@/components/ui/StepProgress';
import { markIntroSeen } from '@/lib/onboarding/intro-seen';
import { space } from '@/lib/theme/tokens';

export default function EveryoneRollsScreen() {
  const router = useRouter();

  function handleContinue() {
    void markIntroSeen();
    router.push('/choose-method');
  }

  return (
    <Screen scroll={false}>
      <FadeInUp delay={180} duration={350} style={{ paddingTop: space.sm }}>
        <StepProgress current={3} total={3} />
      </FadeInUp>

      <ImageReveal delay={440} style={{ marginTop: space.xl, marginBottom: space.xl }}>
        <SplitPhotoPanels />
      </ImageReveal>

      <FadeInUp delay={260} duration={420}>
        <AppText role="display">{'everyone rolls. everyone shares.'}</AppText>
      </FadeInUp>

      <FadeInUp delay={340} duration={400} style={{ marginTop: space.md, marginBottom: space.xl }}>
        <AppText role="body">
          {
            'Every night at 8:00 PM, your circle each rolls their own category from their own wheel. The fun is seeing what everyone got and how they showed up for it.'
          }
        </AppText>
      </FadeInUp>

      <FadeInUp delay={540} duration={400} style={{ marginTop: 'auto', paddingBottom: space.lg }}>
        <PrimaryButton label="Continue" onPress={handleContinue} />
      </FadeInUp>
    </Screen>
  );
}

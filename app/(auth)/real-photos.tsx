import { useRouter } from 'expo-router';
import { View } from 'react-native';
import { FadeInUp, ImageReveal } from '@/lib/motion/primitives';
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
      <AdvanceControl onAdvance={() => router.push('/everyone-spins')}>
        <FadeInUp delay={180} duration={350} style={{ paddingTop: space.sm }}>
          <StepProgress current={2} total={3} />
        </FadeInUp>

        <ImageReveal delay={440} style={{ marginTop: space.xl, marginBottom: space.xl }}>
          <PhotoPanel />
        </ImageReveal>

        <FadeInUp delay={260} duration={420}>
          <AppText role="display">{'real photos only.'}</AppText>
        </FadeInUp>

        <FadeInUp delay={340} duration={400} style={{ marginTop: space.md }}>
          <AppText role="body">
            {'No posters. No album covers. No stock images. Only moments you actually captured.'}
          </AppText>
        </FadeInUp>
      </AdvanceControl>
    </Screen>
  );
}

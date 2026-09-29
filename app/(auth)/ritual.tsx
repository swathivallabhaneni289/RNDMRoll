import { useRouter } from 'expo-router';
import { View } from 'react-native';
import { FadeInUp, ImageReveal } from '@/lib/motion/primitives';
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
        <FadeInUp delay={180} duration={350} style={{ paddingTop: space.sm }}>
          <StepProgress current={1} total={3} />
        </FadeInUp>

        {/* Layout stays as designed (photos above the headline); only the
            entrance TIMING follows docs/motion-interaction-direction.md
            Section 1's heading -> text -> image -> action sequence, via each
            element's own delay -- the photo trio (visually first) is timed
            to settle in last. */}
        <View
          style={{
            flexDirection: 'row',
            gap: space.sm,
            marginTop: space.xl,
            marginBottom: space.xl,
          }}
        >
          <ImageReveal delay={440} style={{ flex: 1, marginTop: space.md }}>
            <PolaroidCard rotation={-6} />
          </ImageReveal>
          <ImageReveal delay={440} style={{ flex: 1 }}>
            <PolaroidCard rotation={0} />
          </ImageReveal>
          <ImageReveal delay={440} style={{ flex: 1, marginTop: space.md }}>
            <PolaroidCard rotation={6} />
          </ImageReveal>
        </View>

        <FadeInUp delay={260} duration={420}>
          <AppText role="display">{"life's better when it's random."}</AppText>
        </FadeInUp>

        <FadeInUp delay={340} duration={400} style={{ marginTop: space.md }}>
          <AppText role="body">
            {"Every night at 8:00 PM, your wheel reveals tonight's category."}
          </AppText>
        </FadeInUp>
      </AdvanceControl>
    </Screen>
  );
}

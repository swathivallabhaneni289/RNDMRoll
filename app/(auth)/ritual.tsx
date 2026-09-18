import { useRouter } from 'expo-router';
import { View } from 'react-native';
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
        <View style={{ paddingTop: space.sm }}>
          <StepProgress current={1} total={3} />
        </View>

        <View
          style={{
            flexDirection: 'row',
            gap: space.sm,
            marginTop: space.xl,
            marginBottom: space.xl,
          }}
        >
          <View style={{ flex: 1, marginTop: space.md }}>
            <PolaroidCard rotation={-6} />
          </View>
          <View style={{ flex: 1 }}>
            <PolaroidCard rotation={0} />
          </View>
          <View style={{ flex: 1, marginTop: space.md }}>
            <PolaroidCard rotation={6} />
          </View>
        </View>

        <AppText role="display">{"life's better when it's random."}</AppText>

        <View style={{ marginTop: space.md }}>
          <AppText role="body">
            {"Every night at 8:00 PM, your wheel reveals tonight's category."}
          </AppText>
        </View>
      </AdvanceControl>
    </Screen>
  );
}

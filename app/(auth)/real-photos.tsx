import { useRouter } from 'expo-router';
import { View } from 'react-native';
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
        <View style={{ paddingTop: space.sm }}>
          <StepProgress current={2} total={3} />
        </View>

        <View style={{ marginTop: space.xl, marginBottom: space.xl }}>
          <PhotoPanel />
        </View>

        <AppText role="display">{'real photos only.'}</AppText>

        <View style={{ marginTop: space.md }}>
          <AppText role="body">
            {'No posters. No album covers. No stock images. Only moments you actually captured.'}
          </AppText>
        </View>
      </AdvanceControl>
    </Screen>
  );
}

import { useRouter } from 'expo-router';
import { View } from 'react-native';
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
      <View style={{ paddingTop: space.sm }}>
        <StepProgress current={3} total={3} />
      </View>

      <View style={{ marginTop: space.xl, marginBottom: space.xl }}>
        <SplitPhotoPanels />
      </View>

      <AppText role="display">{'everyone rolls. everyone shares.'}</AppText>

      <View style={{ marginTop: space.md, marginBottom: space.xl }}>
        <AppText role="body">
          {
            'Every night at 8:00 PM, your circle each rolls their own category from their own wheel. The fun is seeing what everyone got and how they showed up for it.'
          }
        </AppText>
      </View>

      <View style={{ marginTop: 'auto', paddingBottom: space.lg }}>
        <PrimaryButton label="Continue" onPress={handleContinue} />
      </View>
    </Screen>
  );
}

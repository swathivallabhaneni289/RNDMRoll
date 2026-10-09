import { View } from 'react-native';
import { useRouter } from 'expo-router';
import { BrandMark } from '@/components/brand/BrandMark';
import { AppText } from '@/components/ui/AppText';
import { PrimaryButton } from '@/components/ui/PrimaryButton';
import { Screen } from '@/components/ui/Screen';
import { space } from '@/lib/theme/tokens';

/**
 * D-05 step 1: the first screen. This is the phase's one screen carrying
 * the Brand Mark + texture moment (UI-SPEC's Focal Points / Brand Mark /
 * Background Texture sections). It offers a black Sign up button (straight to
 * the sign-up page) and an outlined Log in button (developer, 2026-10-09). D-03:
 * no SMS-based method of any kind.
 */
export default function ChooseMethodScreen() {
  const router = useRouter();

  return (
    <Screen wheels>
      <View style={{ flex: 1, justifyContent: 'center' }}>
        <View style={{ alignItems: 'center' }}>
          <BrandMark size={64} />
          <View style={{ marginTop: space.md }}>
            <AppText role="display">RNDMRoll</AppText>
          </View>
        </View>

        <View style={{ marginTop: space.xxl, gap: space.md }}>
          <PrimaryButton
            label="Sign up"
            onPress={() => router.push('/make-it-yours')}
          />
          <PrimaryButton
            variant="outline"
            label="Log in"
            onPress={() => router.push('/login')}
          />
        </View>
      </View>
    </Screen>
  );
}

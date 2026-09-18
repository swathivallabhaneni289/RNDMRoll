import { useEffect, useState } from 'react';
import { Platform, View } from 'react-native';
import { useRouter } from 'expo-router';
import { BrandMark } from '@/components/brand/BrandMark';
import { AppText } from '@/components/ui/AppText';
import { MethodButton } from '@/components/ui/MethodButton';
import { Screen } from '@/components/ui/Screen';
import { space } from '@/lib/theme/tokens';
import { useSession } from '@/lib/session/store';
import { ApiError } from '@/lib/api/client';
import {
  isAppleSignInAvailable,
  signInWithApple,
  signInWithGoogle,
  SIGN_IN_CANCELED,
} from '@/lib/auth/social';

/**
 * D-05 step 1: the method chooser. This is the phase's one screen carrying
 * the Brand Mark + texture moment (UI-SPEC's Focal Points / Brand Mark /
 * Background Texture sections). D-01/D-02/D-03: exactly three methods,
 * Apple gated to iOS, and no fourth SMS-based method of any kind.
 */
export default function ChooseMethodScreen() {
  const router = useRouter();
  const { signIn } = useSession();
  const [appleAvailable, setAppleAvailable] = useState(false);
  const [inFlight, setInFlight] = useState<'apple' | 'google' | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (Platform.OS !== 'ios') return;
    let cancelled = false;
    isAppleSignInAvailable().then((available) => {
      if (!cancelled) setAppleAvailable(available);
    });
    return () => {
      cancelled = true;
    };
  }, []);

  const showApple = Platform.OS === 'ios' && appleAvailable;

  async function handleSocial(provider: 'apple' | 'google') {
    setError(null);
    setInFlight(provider);
    try {
      const result = await (provider === 'apple' ? signInWithApple() : signInWithGoogle());
      if (result === SIGN_IN_CANCELED) {
        return;
      }
      // Root layout's Stack.Protected guards route onward from here: a brand
      // new social account has no username yet, so onboarding_complete is
      // false and the guard lands on profile-setup, not (app).
      await signIn(result);
    } catch (err) {
      setError(err instanceof ApiError ? err.userMessage : 'Something went wrong. Please try again.');
    } finally {
      setInFlight(null);
    }
  }

  return (
    <Screen texture>
      <View style={{ flex: 1, justifyContent: 'center' }}>
        <View style={{ alignItems: 'center' }}>
          <BrandMark size={64} />
          <View style={{ marginTop: space.md }}>
            <AppText role="display">RNDMRoll</AppText>
          </View>
        </View>

        <View style={{ marginTop: space.xxl, gap: space.md }}>
          <MethodButton
            provider="email"
            label="Continue with Email"
            onPress={() => router.push('/email')}
            disabled={inFlight !== null}
          />
          {showApple ? (
            <MethodButton
              provider="apple"
              label="Continue with Apple"
              onPress={() => handleSocial('apple')}
              loading={inFlight === 'apple'}
              disabled={inFlight !== null && inFlight !== 'apple'}
            />
          ) : null}
          <MethodButton
            provider="google"
            label="Continue with Google"
            onPress={() => handleSocial('google')}
            loading={inFlight === 'google'}
            disabled={inFlight !== null && inFlight !== 'google'}
          />
        </View>

        {error ? (
          <View style={{ marginTop: space.md, alignItems: 'center' }}>
            <AppText role="body" tone="destructive">
              {error}
            </AppText>
          </View>
        ) : null}
      </View>
    </Screen>
  );
}

import { useEffect, useState } from 'react';
import { Platform, View } from 'react-native';
import { useRouter } from 'expo-router';
import { BrandMark } from '@/components/brand/BrandMark';
import { AppText } from '@/components/ui/AppText';
import { MethodButton } from '@/components/ui/MethodButton';
import { PrimaryButton } from '@/components/ui/PrimaryButton';
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
 * D-05 step 1: the first screen. This is the phase's one screen carrying
 * the Brand Mark + texture moment (UI-SPEC's Focal Points / Brand Mark /
 * Background Texture sections). It offers a black Sign up button (straight to
 * the email form) and an outlined Log in button (developer, 2026-10-09). D-03:
 * no SMS-based method of any kind.
 */
// Apple and Google are hidden until their real accounts exist (developer, 2026-10-09: "let's
// hide it ... keep the email part"). Google without its client IDs shows a red error screen
// here and could crash a finished app. Set a constant to true to show that button below the
// Sign up and Log in pair; the code and the server stay as they are.
const APPLE_SIGN_IN_ENABLED = false;
const GOOGLE_SIGN_IN_ENABLED = false;

export default function ChooseMethodScreen() {
  const router = useRouter();
  const { signIn } = useSession();
  const [appleAvailable, setAppleAvailable] = useState(false);
  const [inFlight, setInFlight] = useState<'apple' | 'google' | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!APPLE_SIGN_IN_ENABLED || Platform.OS !== 'ios') return;
    let cancelled = false;
    isAppleSignInAvailable().then((available) => {
      if (!cancelled) setAppleAvailable(available);
    });
    return () => {
      cancelled = true;
    };
  }, []);

  const showApple = APPLE_SIGN_IN_ENABLED && Platform.OS === 'ios' && appleAvailable;

  async function handleSocial(provider: 'apple' | 'google') {
    setError(null);
    setInFlight(provider);
    try {
      const result = await (provider === 'apple' ? signInWithApple() : signInWithGoogle());
      if (result === SIGN_IN_CANCELED) {
        return;
      }
      // Root layout's Stack.Protected guards route onward from here: a brand
      // new social account has no name, username or birthday yet, so
      // onboarding_complete is false and the (auth) layout sends it to the
      // finish page (make-it-yours), not (app).
      await signIn(result);
    } catch (err) {
      setError(err instanceof ApiError ? err.userMessage : 'Something went wrong. Please try again.');
    } finally {
      setInFlight(null);
    }
  }

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
            disabled={inFlight !== null}
          />
          <PrimaryButton
            variant="outline"
            label="Log in"
            onPress={() => router.push('/login')}
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
          {GOOGLE_SIGN_IN_ENABLED ? (
            <MethodButton
              provider="google"
              label="Continue with Google"
              onPress={() => handleSocial('google')}
              loading={inFlight === 'google'}
              disabled={inFlight !== null && inFlight !== 'google'}
            />
          ) : null}
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

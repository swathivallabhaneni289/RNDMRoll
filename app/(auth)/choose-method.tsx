import { useState } from 'react';
import { View } from 'react-native';
import { useRouter } from 'expo-router';
import { BrandMark } from '@/components/brand/BrandMark';
import { AppText, TextLink } from '@/components/ui/AppText';
import { MethodButton } from '@/components/ui/MethodButton';
import { Screen } from '@/components/ui/Screen';
import { space } from '@/lib/theme/tokens';
import { useSession } from '@/lib/session/store';
import { ApiError } from '@/lib/api/client';
import { signInWithGoogle, SIGN_IN_CANCELED } from '@/lib/auth/social';

/**
 * D-05 step 1: the method chooser. This is the phase's one screen carrying
 * the Brand Mark + texture moment (UI-SPEC's Focal Points / Brand Mark /
 * Background Texture sections). Two methods: email and Google. The developer
 * dropped Sign in with Apple on 2026-10-08 (the code in lib/auth/social.ts and
 * the server stay for later). No SMS-based method of any kind (D-03).
 */
export default function ChooseMethodScreen() {
  const router = useRouter();
  const { signIn } = useSession();
  const [inFlight, setInFlight] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleGoogle() {
    setError(null);
    setInFlight(true);
    try {
      const result = await signInWithGoogle();
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
      setInFlight(false);
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
          <MethodButton
            provider="email"
            label="Continue with Email"
            onPress={() => router.push('/make-it-yours')}
            disabled={inFlight}
          />
          <MethodButton
            provider="google"
            label="Continue with Google"
            onPress={handleGoogle}
            loading={inFlight}
          />
        </View>

        {error ? (
          <View style={{ marginTop: space.md, alignItems: 'center' }}>
            <AppText role="body" tone="destructive">
              {error}
            </AppText>
          </View>
        ) : null}

        <View style={{ marginTop: space.lg, alignItems: 'center', flexDirection: 'row', justifyContent: 'center' }}>
          <AppText role="body" tone="muted">
            {'Already have an account? '}
          </AppText>
          <TextLink onPress={() => router.push('/login')}>Log in</TextLink>
        </View>
      </View>
    </Screen>
  );
}

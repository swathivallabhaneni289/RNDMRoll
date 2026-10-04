import { useEffect } from 'react';
import { Stack, useRouter } from 'expo-router';
import { color } from '@/lib/theme/tokens';
import { useSession } from '@/lib/session/store';

/**
 * D-05 (revised 2026-10-04) sequence order. Welcome is the single pre-signup
 * page (UI-SPEC revision 11; it replaced the four-screen marketing sequence of
 * plan 01-16); the last is the consolidated create-profile screen (plan 01-12),
 * which replaces the prior separate name/username/photo routes. 'welcome' is the
 * file-system default entry (see unstable_settings below) for a first-ever cold
 * start; the effect below redirects away from it for the other two cases this
 * layout owns.
 */
export const unstable_settings = {
  initialRouteName: 'welcome',
};

export default function AuthLayout() {
  const router = useRouter();
  const { status, user } = useSession();

  const fadeTransition = { animation: 'fade' as const, animationDuration: 220 };

  // A half-onboarded authenticated user always resumes at profile-setup. Everyone
  // else starts on the default initial route, 'welcome', on every launch (UI-SPEC
  // revision 11): there is no skip-after-first-time rule any more, so the first
  // page is always the same and there is no flash of the sign-in page before it.
  useEffect(() => {
    if (status === 'authenticated' && user?.onboarding_complete === false) {
      router.replace('/profile-setup');
    }
  }, [status, user?.onboarding_complete, router]);

  return (
    <Stack
      screenOptions={{
        headerShown: false,
        contentStyle: { backgroundColor: color.dominant },
      }}
    >
      <Stack.Screen name="welcome" />
      <Stack.Screen name="choose-method" options={fadeTransition} />
      <Stack.Screen name="email" />
      <Stack.Screen name="verify-email" />
      <Stack.Screen name="profile-setup" />
    </Stack>
  );
}

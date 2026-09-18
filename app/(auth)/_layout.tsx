import { useEffect } from 'react';
import { Stack, useRouter } from 'expo-router';
import { color } from '@/lib/theme/tokens';
import { useSession } from '@/lib/session/store';
import { useIntroSeen } from '@/lib/onboarding/intro-seen';

/**
 * D-05 (revised 2026-09-17) sequence order. The first four are the pre-signup
 * marketing screens (plan 01-16); the last is the consolidated create-profile
 * screen (plan 01-12), which replaces the prior separate name/username/photo
 * routes. 'welcome' is the file-system default entry (see unstable_settings
 * below) for a first-ever cold start; the effect below redirects away from it
 * for the other two cases this layout owns.
 */
export const unstable_settings = {
  initialRouteName: 'welcome',
};

export default function AuthLayout() {
  const router = useRouter();
  const { status, user } = useSession();
  const { seenIntro, resolved: introResolved } = useIntroSeen();

  // The initial route is decided from two inputs and nothing else, per the plan:
  // a half-onboarded authenticated user always resumes at profile-setup, and an
  // unauthenticated user's start point depends on whether they've already seen
  // the pre-signup marketing sequence.
  useEffect(() => {
    if (status === 'authenticated' && user?.onboarding_complete === false) {
      router.replace('/profile-setup');
      return;
    }
    if (!introResolved) return;
    if (seenIntro === true) {
      router.replace('/choose-method');
    }
    // seenIntro === false: stay on the default initial route, 'welcome'.
  }, [status, user?.onboarding_complete, introResolved, seenIntro, router]);

  return (
    <Stack
      screenOptions={{
        headerShown: false,
        contentStyle: { backgroundColor: color.dominant },
      }}
    >
      <Stack.Screen name="welcome" />
      <Stack.Screen name="ritual" />
      <Stack.Screen name="real-photos" />
      <Stack.Screen name="everyone-rolls" />
      <Stack.Screen name="choose-method" />
      <Stack.Screen name="email" />
      <Stack.Screen name="verify-email" />
      <Stack.Screen name="profile-setup" />
    </Stack>
  );
}

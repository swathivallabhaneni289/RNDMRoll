import { useEffect } from 'react';
import { Stack, useRouter } from 'expo-router';
import { useReducedMotion } from 'react-native-reanimated';
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
  const reducedMotion = useReducedMotion();

  // Button-advanced screens (reached by tapping a CTA, not by the AdvanceControl
  // swipe/chevron) fade in rather than push, so the direction change between the
  // two advance mechanisms reads intentionally rather than identically. Reduced
  // Motion collapses both to a fade per Apple's own guidance: never delete
  // meaning-bearing motion outright, substitute a dissolve.
  const pushTransition = reducedMotion
    ? { animation: 'fade' as const, animationDuration: 220 }
    : { animation: 'simple_push' as const, animationDuration: 220 };
  const fadeTransition = { animation: 'fade' as const, animationDuration: 220 };

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
      <Stack.Screen name="ritual" options={fadeTransition} />
      <Stack.Screen name="real-photos" options={pushTransition} />
      <Stack.Screen name="everyone-spins" options={pushTransition} />
      <Stack.Screen name="choose-method" options={fadeTransition} />
      <Stack.Screen name="email" />
      <Stack.Screen name="verify-email" />
      <Stack.Screen name="profile-setup" />
    </Stack>
  );
}

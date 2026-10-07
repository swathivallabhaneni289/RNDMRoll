import { useEffect } from 'react';
import { BackHandler } from 'react-native';
import { Stack, useRouter, useSegments } from 'expo-router';
import { color } from '@/lib/theme/tokens';
import { useSession } from '@/lib/session/store';

/**
 * The signed-out screens, in the order a person meets them. Welcome is the single
 * pre-signup page (UI-SPEC revision 11); choose-method picks email, Apple or Google;
 * make-it-yours is the one-page sign-up (and the finish page for an unfinished Apple or
 * Google account, plan 01-19); login is the email log-in. 'welcome' is the file-system
 * default entry (see unstable_settings below) for every cold start; the effect below
 * redirects away from it for an unfinished signed-in account.
 */
export const unstable_settings = {
  initialRouteName: 'welcome',
};

export default function AuthLayout() {
  const router = useRouter();
  const segments = useSegments();
  const { status, user } = useSession();

  const fadeTransition = { animation: 'fade' as const, animationDuration: 220 };

  const signedIn = status === 'authenticated';
  const onMakeItYours = (segments as string[])[segments.length - 1] === 'make-it-yours';

  // A signed-in account that is still unfinished always resumes at make-it-yours (finish
  // mode). Everyone else starts on the default initial route, 'welcome', on every launch
  // (UI-SPEC revision 11): there is no skip-after-first-time rule, so the first page is
  // always the same. Already being on the page is not a reason to replace it: that would
  // remount the form and drop what was typed.
  useEffect(() => {
    if (signedIn && user?.onboarding_complete === false && !onMakeItYours) {
      router.replace('/make-it-yours');
    }
  }, [signedIn, user?.onboarding_complete, onMakeItYours, router]);

  // gestureEnabled covers the iOS swipe only; Android's hardware back ignores it. A
  // signed-in person on make-it-yours must not back out to Welcome or choose-method
  // (and drop what they typed), so there back leaves the app instead of popping the
  // stack. Signed out, the sign-up form is an ordinary screen and back works as usual.
  const trapBack = signedIn && onMakeItYours;
  useEffect(() => {
    if (!trapBack) return;
    const subscription = BackHandler.addEventListener('hardwareBackPress', () => {
      BackHandler.exitApp();
      return true;
    });
    return () => subscription.remove();
  }, [trapBack]);

  return (
    <Stack
      screenOptions={{
        headerShown: false,
        contentStyle: { backgroundColor: color.dominant },
      }}
    >
      <Stack.Screen name="welcome" />
      <Stack.Screen name="choose-method" options={fadeTransition} />
      <Stack.Screen name="make-it-yours" options={{ gestureEnabled: !signedIn }} />
      <Stack.Screen name="login" />
    </Stack>
  );
}

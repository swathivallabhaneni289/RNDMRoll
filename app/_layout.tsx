import { useEffect } from 'react';
import { Stack } from 'expo-router';
import * as SplashScreen from 'expo-splash-screen';
import { SessionProvider, useSession } from '@/lib/session/store';
import { useAppFonts } from '@/lib/theme/fonts';
import { useIntroSeen } from '@/lib/onboarding/intro-seen';

// Held until the launch gate below explicitly hides it, once fonts, the session
// restore, and the intro flag have all settled — never sooner, so no wrong first
// screen ever flashes before the route decision is ready.
SplashScreen.preventAutoHideAsync();

function RootNavigator() {
  const { status, user } = useSession();
  const { fontsLoaded, fontError } = useAppFonts();
  const { resolved: introResolved } = useIntroSeen();

  const fontsSettled = fontsLoaded || fontError;
  const sessionSettled = status !== 'loading';
  const gateSettled = sessionSettled && fontsSettled && introResolved;

  useEffect(() => {
    if (gateSettled) {
      SplashScreen.hideAsync();
    }
  }, [gateSettled]);

  if (!gateSettled) {
    return null;
  }

  return (
    <Stack screenOptions={{ headerShown: false }}>
      <Stack.Protected guard={status === 'authenticated' && user?.onboarding_complete === true}>
        <Stack.Screen name="(app)" />
      </Stack.Protected>
      <Stack.Protected guard={status !== 'authenticated' || user?.onboarding_complete === false}>
        <Stack.Screen name="(auth)" />
      </Stack.Protected>
    </Stack>
  );
}

export default function RootLayout() {
  return (
    <SessionProvider>
      <RootNavigator />
    </SessionProvider>
  );
}

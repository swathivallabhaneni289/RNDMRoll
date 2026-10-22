import { useEffect } from 'react';
import { StyleSheet, View } from 'react-native';
import { Stack } from 'expo-router';
import * as SplashScreen from 'expo-splash-screen';
import { OfflineScreen } from '@/components/session/OfflineScreen';
import { clearResetTries } from '@/lib/auth/clearResetTries';
import { SessionProvider, useSession } from '@/lib/session/store';
import { useAppFonts } from '@/lib/theme/fonts';
import { useIntroSeen } from '@/lib/onboarding/intro-seen';

// Held until the launch gate below explicitly hides it, once fonts, the session
// restore, and the intro flag have all settled, never sooner, so no wrong first
// screen ever flashes before the route decision is ready.
SplashScreen.preventAutoHideAsync();

// In the background, once (a mark remembers it ran): the Forgot password count an earlier build
// kept on the phone.
clearResetTries();

// Mirrors (auth)/_layout.tsx's own unstable_settings as a defensive default
// now that "index" and the two Stack.Protected groups sit side by side. It did
// not fix the Unmatched Route on cold launch (tested, no change); the explicit
// child redirects in app/index.tsx did, so do not rely on this pin for that.
export const unstable_settings = {
  initialRouteName: 'index',
};

function RootNavigator() {
  const { status, user } = useSession();
  const { fontsLoaded, fontError } = useAppFonts();
  const { resolved: introResolved } = useIntroSeen();

  const fontsSettled = fontsLoaded || fontError;
  const sessionSettled = status !== 'loading';
  const gateSettled = Boolean(sessionSettled && fontsSettled && introResolved);

  useEffect(() => {
    if (gateSettled) {
      SplashScreen.hideAsync();
    }
  }, [gateSettled]);

  // The Stack (and the "index" route inside it) must always render, even
  // before gateSettled: expo-router's linking resolver matches the initial
  // URL against whatever screens exist on the very first render. Gating the
  // whole Stack behind a `return null` here (the previous approach) meant
  // that first match always failed with no Stack to match against, and
  // expo-router's built-in Unmatched Route screen stuck permanently even
  // once the Stack mounted for real. The native splash screen (still up
  // until the effect above calls hideAsync()) covers this visually, so
  // gating on `gateSettled` inside each guard is enough: nothing wrong
  // flashes, but the route tree exists from the first render onward.
  return (
    <>
    <Stack screenOptions={{ headerShown: false }}>
      {/* Unconditional: see app/index.tsx for why this route needs to always
          resolve, independent of the Protected guards below. */}
      <Stack.Screen name="index" />
      <Stack.Protected guard={gateSettled && status === 'authenticated' && user?.onboarding_complete === true}>
        <Stack.Screen name="(app)" />
      </Stack.Protected>
      <Stack.Protected guard={gateSettled && (status !== 'authenticated' || user?.onboarding_complete === false)}>
        <Stack.Screen name="(auth)" />
      </Stack.Protected>
    </Stack>
    {/* Tokens are stored but the server could not be reached: not signed out, so no Welcome page. */}
    {gateSettled && status === 'offline' ? (
      <View style={StyleSheet.absoluteFill}>
        <OfflineScreen />
      </View>
    ) : null}
    </>
  );
}

export default function RootLayout() {
  return (
    <SessionProvider>
      <RootNavigator />
    </SessionProvider>
  );
}

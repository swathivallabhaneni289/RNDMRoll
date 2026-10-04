import { Redirect } from 'expo-router';
import { useSession } from '@/lib/session/store';
import { useAppFonts } from '@/lib/theme/fonts';
import { useIntroSeen } from '@/lib/onboarding/intro-seen';

/**
 * expo-development-client's initial bundle handshake resolves to the literal
 * "/" path, which this app never otherwise registers a route for; every
 * real screen lives inside the (app) or (auth) group, selected by
 * RootNavigator's Stack.Protected guards in _layout.tsx, not by an index
 * route. Without a literal index route to land on first, the dev client
 * shows expo-router's built-in Unmatched Route screen instead of ever
 * reaching RootNavigator's own routing. This file exists solely to give
 * that handshake something to resolve to; the redirect below intentionally
 * mirrors _layout.tsx's Stack.Protected guard conditions exactly rather
 * than introducing a second source of truth for the decision.
 *
 * This component re-derives the same gateSettled check RootNavigator uses
 * (rather than importing it from there, since there's no shared context for it)
 * because this route, unlike (app)/(auth), is unconditionally mounted from
 * the very first render, before session/fonts/intro-seen have resolved.
 * Redirecting before they settle would send an about-to-be-authenticated
 * user through (auth) for a flash, which is exactly what _layout.tsx's
 * gating exists to prevent.
 *
 * Redirects target each group's own explicit initial screen ("/(app)/profile",
 * "/(auth)/welcome") rather than the bare group path ("/(app)", "/(auth)").
 * Empirically, a bare group path fails to resolve here and falls through to
 * Unmatched Route: the group's own Stack.Protected guard and this redirect
 * both fire from the same gateSettled flip, and the guard hasn't committed
 * yet when the bare-group redirect tries to resolve against it. An explicit
 * child path doesn't depend on that timing. Welcome is the start page on every
 * signed-out launch (UI-SPEC revision 11), so there is no second redirect and
 * no flash of the sign-in page before it.
 */
export default function Index() {
  const { status, user } = useSession();
  const { fontsLoaded, fontError } = useAppFonts();
  const { resolved: introResolved } = useIntroSeen();

  const gateSettled = Boolean(status !== 'loading' && (fontsLoaded || fontError) && introResolved);

  if (!gateSettled) {
    // The native splash screen is still covering the app at this point
    // (RootNavigator only calls SplashScreen.hideAsync() once gateSettled),
    // so this is never visibly blank.
    return null;
  }

  if (status === 'authenticated' && user?.onboarding_complete === true) {
    return <Redirect href="/(app)/profile" />;
  }
  return <Redirect href="/(auth)/welcome" />;
}

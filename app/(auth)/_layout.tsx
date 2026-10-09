import { Stack } from 'expo-router';
import { color } from '@/lib/theme/tokens';
import { useSession } from '@/lib/session/store';

/**
 * The signed-out screens, in the order a person meets them. Welcome is the single
 * pre-signup page (UI-SPEC revision 11); choose-method offers Sign up and Log in;
 * make-it-yours is the sign-up page; login is the log-in page. 'welcome' is the file-system
 * default entry (see unstable_settings below) for every cold start.
 */
export const unstable_settings = {
  initialRouteName: 'welcome',
};

export default function AuthLayout() {
  const { status } = useSession();

  const fadeTransition = { animation: 'fade' as const, animationDuration: 220 };

  const signedIn = status === 'authenticated';

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

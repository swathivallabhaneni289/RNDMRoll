import { Redirect, Stack } from 'expo-router';
import { GestureHandlerRootView } from 'react-native-gesture-handler';
import { color } from '@/lib/theme/tokens';

// Same reason as the root and (auth) layouts: give the linking resolver a default
// screen for the bare "/lab" URL.
export const unstable_settings = {
  initialRouteName: 'index',
};

/**
 * Layout for the frosted-material lab. Development builds only: in a release bundle
 * every lab URL redirects away. expo-router does not provide a gesture root, and the
 * entry detail's pull-down dismiss needs one, so it is added here.
 *
 * The entry screen is a transparent modal with no native animation: it runs its own
 * shared-photo transition, and the list it was opened from must stay visible behind
 * it while it does.
 */
export default function LabLayout() {
  if (!__DEV__) return <Redirect href="/" />;

  return (
    <GestureHandlerRootView style={{ flex: 1 }}>
      <Stack screenOptions={{ headerShown: false, contentStyle: { backgroundColor: color.dominant } }}>
        <Stack.Screen name="index" />
        <Stack.Screen name="feed" />
        <Stack.Screen name="diary" />
        <Stack.Screen
          name="entry/[id]"
          options={{
            presentation: 'transparentModal',
            animation: 'none',
            gestureEnabled: false,
            contentStyle: { backgroundColor: 'transparent' },
          }}
        />
      </Stack>
    </GestureHandlerRootView>
  );
}

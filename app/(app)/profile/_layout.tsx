import { Stack } from 'expo-router';
import { color } from '@/lib/theme/tokens';

/**
 * ACCT-03/D-08's two profile routes (view, edit). This file is what turns
 * "profile" into a real nested-layout route name matching the parent
 * (app)/_layout.tsx's `<Stack.Screen name="profile" />` declaration:
 * expo-router hoists routes in a directory without a _layout file directly
 * into the nearest ancestor layout (as "profile/index" and "profile/edit"),
 * which would leave that parent declaration unmatched. With this file
 * present, "profile" becomes its own nested Stack, and index/edit push and
 * pop within it the way edit.tsx's `router.back()` expects.
 */
export default function ProfileLayout() {
  return (
    <Stack
      screenOptions={{
        headerShown: false,
        contentStyle: { backgroundColor: color.dominant },
      }}
    >
      <Stack.Screen name="index" />
      <Stack.Screen name="edit" />
    </Stack>
  );
}

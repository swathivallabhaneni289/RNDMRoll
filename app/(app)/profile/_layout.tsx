import { Stack } from 'expo-router';
import { color } from '@/lib/theme/tokens';

/**
 * ACCT-03/D-08's two profile routes (view, edit), and Phase 2.1's three
 * Settings pages (settings, username, password; plan 02.1-02). This file is
 * what turns "profile" into a real nested-layout route name matching the
 * parent (app)/_layout.tsx's `<Stack.Screen name="profile" />` declaration:
 * expo-router hoists routes in a directory without a _layout file directly
 * into the nearest ancestor layout (as "profile/index" and "profile/edit"),
 * which would leave that parent declaration unmatched. With this file
 * present, "profile" becomes its own nested Stack, and index/edit and the
 * three Settings pages push and pop within it the way edit.tsx's
 * `router.back()` expects, with the bottom bar showing on each of them.
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
      <Stack.Screen name="settings" />
      <Stack.Screen name="username" />
      <Stack.Screen name="password" />
    </Stack>
  );
}

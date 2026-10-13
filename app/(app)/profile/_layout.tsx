import { Stack } from 'expo-router';
import { color } from '@/lib/theme/tokens';

/**
 * The Profile tab's stack: the Profile page (index), the Edit page (edit), and Settings with its
 * pages (settings, notification-settings, blocked-accounts, password). The username and the bio
 * are edited on the Profile page and its Edit page, so Settings has no page for them (decision
 * D-16 of Phase 2.1). This file is what turns "profile" into a real nested-layout route name
 * matching the parent (app)/_layout.tsx's `<Stack.Screen name="profile" />` declaration:
 * expo-router hoists routes in a directory without a _layout file directly into the nearest
 * ancestor layout, which would leave that parent declaration unmatched. With this file present,
 * "profile" becomes its own nested Stack, and every page here pushes and pops within it, with the
 * bottom bar showing on each of them.
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
      <Stack.Screen name="notification-settings" />
      <Stack.Screen name="blocked-accounts" />
      <Stack.Screen name="password" />
    </Stack>
  );
}

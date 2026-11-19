import { Stack } from 'expo-router';
import { color } from '@/lib/theme/tokens';

/**
 * Home is a Stack inside a tab: Today at the bottom, with Edit wheel and the Notifications page
 * pushed over it (plan 02-02 decision 3). The bottom bar stays on Edit wheel and is hidden on
 * Notifications; the Tabs layout above does that by reading this stack's focused route name.
 * Phase 3 adds the entry screens here: log (Log it), entry (the entry page), edit-entry. A removed
 * post's or comment's line in Notifications pushes second-look, its page to ask for a second look
 * (2026-11-19).
 */
export default function TodayLayout() {
  return (
    <Stack screenOptions={{ headerShown: false, contentStyle: { backgroundColor: color.dominant } }}>
      <Stack.Screen name="index" />
      <Stack.Screen name="edit-wheel" />
      <Stack.Screen name="notifications" />
      <Stack.Screen name="follow-requests" />
      <Stack.Screen name="second-look" />
      <Stack.Screen name="find-friends" />
      <Stack.Screen name="post" />
      <Stack.Screen name="person" />
      <Stack.Screen name="person-people" />
      <Stack.Screen name="log" />
      <Stack.Screen name="entry" />
      <Stack.Screen name="edit-entry" />
    </Stack>
  );
}

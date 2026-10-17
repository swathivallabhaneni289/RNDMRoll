import { Stack } from 'expo-router';
import { color } from '@/lib/theme/tokens';

/**
 * The Board tab's stack (Phase 5): the Board (index), someone else's post (post), your own entry
 * page and edit entry (entry, edit-entry; Back reads 'Board'), People (people), Search (search), someone's
 * profile (person, the other session's Phase 6 person page) and Notifications
 * from the Board's bell (notifications), all pushed over it so Back always returns to the Board.
 * The Tabs layout hides the bottom bar on post, entry, edit-entry and notifications by reading
 * this stack's focused route name; People keeps the bar, as it does in the Profile stack.
 */
export default function BoardLayout() {
  return (
    <Stack screenOptions={{ headerShown: false, contentStyle: { backgroundColor: color.dominant } }}>
      <Stack.Screen name="index" />
      <Stack.Screen name="post" />
      <Stack.Screen name="entry" />
      <Stack.Screen name="edit-entry" />
      <Stack.Screen name="people" />
      <Stack.Screen name="search" />
      <Stack.Screen name="person" />
    </Stack>
  );
}

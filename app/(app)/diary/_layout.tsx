import { Stack } from 'expo-router';
import { color } from '@/lib/theme/tokens';

/**
 * The Diary tab's stack (04-UI-SPEC 3.7): the list (index), the entry page (entry) and edit entry
 * (edit-entry) pushed over it, so opening a post never switches tabs. The pages push with the
 * platform's own slide. The Tabs layout hides the bottom bar on `entry` and `edit-entry` by
 * reading this stack's focused route name, as it does under Today.
 */
export default function DiaryLayout() {
  return (
    <Stack screenOptions={{ headerShown: false, contentStyle: { backgroundColor: color.dominant } }}>
      <Stack.Screen name="index" />
      <Stack.Screen name="entry" />
      <Stack.Screen name="edit-entry" />
    </Stack>
  );
}

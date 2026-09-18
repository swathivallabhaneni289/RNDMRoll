import { Stack } from 'expo-router';
import { color } from '@/lib/theme/tokens';

/**
 * Phase 1 has no bottom tab bar (UI-SPEC's Color section notes it arrives in a
 * later phase; D-07 keeps this phase's app surface to the profile screens only),
 * so this layout declares just the profile route group.
 */
export default function AppLayout() {
  return (
    <Stack screenOptions={{ contentStyle: { backgroundColor: color.dominant } }}>
      <Stack.Screen name="profile" />
    </Stack>
  );
}

import { Pressable } from 'react-native';
import { useRouter } from 'expo-router';
import { Ionicons } from '@expo/vector-icons';
import { color, minTouchTarget } from '@/lib/theme/tokens';

/**
 * The gear that opens Settings (02.1-UI-SPEC section 3): a 24dp outline gear in a 44dp square.
 * No props. The Profile page mounts it at the right end of its header row (in the tab only), and
 * Phase 4's Profile page mounts the same component above its photo row.
 */
export function SettingsGear() {
  const router = useRouter();
  return (
    <Pressable
      onPress={() => router.push('/(app)/profile/settings')}
      accessibilityRole="button"
      accessibilityLabel="Settings"
      accessibilityHint="Opens your settings."
      style={{
        width: minTouchTarget,
        height: minTouchTarget,
        alignItems: 'center',
        justifyContent: 'center',
      }}
    >
      <Ionicons name="settings-outline" size={24} color={color.ink} />
    </Pressable>
  );
}

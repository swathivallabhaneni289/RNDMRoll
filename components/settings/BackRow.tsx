import { Pressable, View } from 'react-native';
import { useRouter } from 'expo-router';
import { Ionicons } from '@expo/vector-icons';
import { AppText } from '@/components/ui/AppText';
import { color, minTouchTarget } from '@/lib/theme/tokens';

/**
 * The Back control of the Settings pages: the one the profile form draws (chevron and "Back"),
 * in a 44dp row at the top of the page. It goes back one page in the Profile stack.
 */
export function BackRow() {
  const router = useRouter();

  function goBack() {
    // A page opened with nothing behind it has nowhere to go back to: land on the Profile page.
    if (router.canGoBack()) router.back();
    else router.replace('/(app)/profile');
  }

  return (
    <View style={{ minHeight: minTouchTarget, flexDirection: 'row', alignItems: 'center' }}>
      <Pressable
        onPress={goBack}
        accessibilityRole="button"
        accessibilityLabel="Back"
        hitSlop={12}
        style={{ minHeight: minTouchTarget, flexDirection: 'row', alignItems: 'center' }}
      >
        <Ionicons name="chevron-back" size={22} color={color.ink} />
        <AppText role="body">Back</AppText>
      </Pressable>
    </View>
  );
}

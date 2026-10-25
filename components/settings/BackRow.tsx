import { Pressable, View } from 'react-native';
import { useRouter, type Href } from 'expo-router';
import { Ionicons } from '@expo/vector-icons';
import { AppText } from '@/components/ui/AppText';
import { color, minTouchTarget } from '@/lib/theme/tokens';

/**
 * The Back control of the Settings pages: the one the profile form draws (chevron and "Back"),
 * in a 44dp row at the top of the page. It goes back one page in the Profile stack. `fallback` is
 * where a page opened with nothing behind it lands: the Profile page, unless the page names another
 * (the signed-out Terms and Privacy pages land on Welcome).
 */
export function BackRow({ fallback = '/(app)/profile' }: { fallback?: Href }) {
  const router = useRouter();

  function goBack() {
    // A page opened with nothing behind it has nowhere to go back to.
    if (router.canGoBack()) router.back();
    else router.replace(fallback);
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

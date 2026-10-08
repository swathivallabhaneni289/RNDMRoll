import { Pressable, View } from 'react-native';
import { useRouter } from 'expo-router';
import { Ionicons } from '@expo/vector-icons';
import { AppText } from '@/components/ui/AppText';
import { Screen } from '@/components/ui/Screen';
import { color, space } from '@/lib/theme/tokens';

/**
 * A plain stand-in for the next phase. Continue on the profile page leads here, and the daily
 * spin page (Phase 2) will replace it. It holds no data and no actions.
 */
export default function HomeScreen() {
  const router = useRouter();

  function back() {
    if (router.canGoBack()) router.back();
    else router.replace('/profile');
  }

  return (
    <Screen wheels="corner">
      <View style={{ paddingTop: space.lg }}>
        <Pressable
          onPress={back}
          accessibilityRole="button"
          accessibilityLabel="Back"
          hitSlop={12}
          style={{ flexDirection: 'row', alignItems: 'center', alignSelf: 'flex-start' }}
        >
          <Ionicons name="chevron-back" size={22} color={color.ink} />
          <AppText role="body">Back</AppText>
        </Pressable>

        <View style={{ marginTop: space.xl }}>
          <AppText role="display">You're in.</AppText>
          <View style={{ marginTop: space.sm }}>
            <AppText role="body" tone="muted">
              Your daily spin will appear here.
            </AppText>
          </View>
        </View>
      </View>
    </Screen>
  );
}

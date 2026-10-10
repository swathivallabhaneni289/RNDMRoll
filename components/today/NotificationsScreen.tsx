import { Pressable, Text, View } from 'react-native';
import { useRouter } from 'expo-router';
import { Ionicons } from '@expo/vector-icons';
import { AppText } from '@/components/ui/AppText';
import { Screen } from '@/components/ui/Screen';
import { color, minTouchTarget, space, type } from '@/lib/theme/tokens';

/**
 * The page behind the bell (02-UI-SPEC 4.5). Phase 2 has nothing to notify about, so the page
 * holds only its heading and one line; the real list (friend activity, comments) and phone
 * alerts are Phase 5. It is there so the bell is never a dead control. No list, no button, no
 * state of its own. It has no bottom bar (the Today tab hides it while this page is focused),
 * so its screen keeps all four safe-area edges.
 */
export function NotificationsScreen() {
  const router = useRouter();

  function goBack() {
    if (router.canGoBack()) router.back();
    else router.replace('/(app)/today');
  }

  return (
    <Screen scroll={false}>
      <View style={{ paddingTop: space.sm }}>
        <Pressable
          onPress={goBack}
          accessibilityRole="button"
          accessibilityLabel="Back"
          hitSlop={8}
          style={{
            width: minTouchTarget,
            height: minTouchTarget,
            justifyContent: 'center',
            alignItems: 'flex-start',
          }}
        >
          <Ionicons name="chevron-back" size={28} color={color.ink} />
        </Pressable>

        <View style={{ marginTop: space.md }}>
          <AppText role="title" accessibilityRole="header">
            Notifications
          </AppText>
        </View>
        <View style={{ marginTop: space.sm }}>
          <Text
            style={{
              fontFamily: type.label.fontFamily,
              fontSize: type.label.fontSize,
              lineHeight: type.label.lineHeight,
              color: color.muted,
            }}
          >
            Nothing new yet.
          </Text>
        </View>
      </View>
    </Screen>
  );
}

import { useEffect, useState } from 'react';
import { AccessibilityInfo, View } from 'react-native';
import { useRouter } from 'expo-router';
import { AppText } from '@/components/ui/AppText';
import { Screen } from '@/components/ui/Screen';
import { BackRow } from '@/components/settings/BackRow';
import { LogOutSheet } from '@/components/settings/LogOutSheet';
import { SettingsRow } from '@/components/settings/SettingsRow';
import { useSettingsNotice } from '@/lib/settings/notice';
import { color, space } from '@/lib/theme/tokens';

/**
 * The Settings page (02.1-UI-SPEC section 4, as changed by decision D-16), pushed over the Profile
 * page with the bottom bar showing: Back, the title, the notice line, then the rows in this order:
 * Notifications, Blocked accounts, Archive, Password and Log out.
 *
 * The bio and the username are not here: they live on the Profile page and its Edit page (D-16).
 * The bar adds the bottom safe-area inset itself, so the screen takes the top and side edges only.
 */
export function SettingsScreen() {
  const router = useRouter();
  const notice = useSettingsNotice();
  const [loggingOut, setLoggingOut] = useState(false);

  // iOS does not read a live region by itself: say the notice once, when it appears.
  useEffect(() => {
    if (notice !== '') AccessibilityInfo.announceForAccessibility(notice);
  }, [notice]);

  return (
    <Screen edges={['top', 'left', 'right']}>
      <View style={{ paddingTop: space.lg, paddingBottom: space.xl }}>
        <BackRow />

        <View style={{ marginTop: space.md }}>
          <AppText role="title" accessibilityRole="header">
            Settings
          </AppText>
        </View>

        {/* The notice line takes room only while there is a notice (developer, 2026-10-18: no empty gap under the title). */}
        <View accessibilityLiveRegion="polite">
          {notice !== '' ? (
            <View style={{ marginTop: space.sm }}>
              <AppText role="label" tone="muted">
                {notice}
              </AppText>
            </View>
          ) : null}
        </View>

        <View
          style={{
            marginTop: space.md,
            borderTopWidth: 1,
            borderTopColor: color.divider,
          }}
        >
          <SettingsRow
            label="Notifications"
            accessibilityHint="Opens the page."
            onPress={() => router.push('/(app)/profile/notification-settings')}
          />
          <SettingsRow
            label="Blocked accounts"
            accessibilityHint="Opens the page."
            onPress={() => router.push('/(app)/profile/blocked-accounts')}
          />
          <SettingsRow
            label="Archive"
            accessibilityHint="Opens the page."
            onPress={() => router.push('/(app)/profile/archive')}
          />
          <SettingsRow
            label="Password"
            accessibilityHint="Opens the page."
            onPress={() => router.push('/(app)/profile/password')}
          />
          <SettingsRow
            label="Log out"
            tone="destructive"
            chevron={false}
            accessibilityHint="Asks first."
            onPress={() => setLoggingOut(true)}
          />
        </View>
      </View>

      <LogOutSheet visible={loggingOut} onClose={() => setLoggingOut(false)} />
    </Screen>
  );
}

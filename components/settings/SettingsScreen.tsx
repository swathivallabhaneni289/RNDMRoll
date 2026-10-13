import { useEffect, useState } from 'react';
import { AccessibilityInfo, View } from 'react-native';
import { useRouter } from 'expo-router';
import { AppText } from '@/components/ui/AppText';
import { Screen } from '@/components/ui/Screen';
import { BackRow } from '@/components/settings/BackRow';
import { BioInline } from '@/components/settings/BioInline';
import { LogOutSheet } from '@/components/settings/LogOutSheet';
import { PrivateAccountRow } from '@/components/settings/PrivateAccountRow';
import { SettingsRow } from '@/components/settings/SettingsRow';
import { useSettingsNotice } from '@/lib/settings/notice';
import { useSession } from '@/lib/session/store';
import { color, space } from '@/lib/theme/tokens';

/**
 * The Settings page (02.1-UI-SPEC section 4), pushed over the Profile page with the bottom bar
 * showing: Back, the title, the notice line, the bio (edited in place), then the rows: Edit
 * profile, Username, Password, Private account and Log out. The bar adds the bottom safe-area
 * inset itself, so the screen takes the top and side edges only.
 */
export function SettingsScreen() {
  const router = useRouter();
  const { user } = useSession();
  const notice = useSettingsNotice();
  const [loggingOut, setLoggingOut] = useState(false);

  // iOS does not read a live region by itself: say the notice once, when it appears.
  useEffect(() => {
    if (notice !== '') AccessibilityInfo.announceForAccessibility(notice);
  }, [notice]);

  const username = user?.username ?? '';

  return (
    <Screen edges={['top', 'left', 'right']}>
      <View style={{ paddingTop: space.lg, paddingBottom: space.xl }}>
        <BackRow />

        <View style={{ marginTop: space.md }}>
          <AppText role="title" accessibilityRole="header">
            Settings
          </AppText>
        </View>

        {/* The notice line keeps its 20dp whether or not there is a notice, so nothing jumps. */}
        <View accessibilityLiveRegion="polite" style={{ marginTop: space.sm, minHeight: 20 }}>
          {notice !== '' ? (
            <AppText role="label" tone="muted">
              {notice}
            </AppText>
          ) : null}
        </View>

        <View style={{ marginTop: space.lg }}>
          <BioInline label />
        </View>

        <View
          style={{
            marginTop: space.lg,
            borderTopWidth: 1,
            borderTopColor: color.divider,
          }}
        >
          <SettingsRow
            label="Edit profile"
            accessibilityHint="Opens the page."
            onPress={() => router.push('/(app)/profile/edit')}
          />
          <SettingsRow
            label="Username"
            value={username !== '' ? `@${username}` : undefined}
            accessibilityLabel={username !== '' ? `Username, ${username}` : 'Username'}
            accessibilityHint="Opens the page."
            onPress={() => router.push('/(app)/profile/username')}
          />
          <SettingsRow
            label="Password"
            accessibilityHint="Opens the page."
            onPress={() => router.push('/(app)/profile/password')}
          />
          <PrivateAccountRow />
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

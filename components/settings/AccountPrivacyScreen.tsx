import { View } from 'react-native';
import { AppText } from '@/components/ui/AppText';
import { Screen } from '@/components/ui/Screen';
import { BackRow } from '@/components/settings/BackRow';
import { PrivateAccountRow } from '@/components/settings/PrivateAccountRow';
import { color, space } from '@/lib/theme/tokens';

/**
 * Settings > Account privacy, pushed from the Account privacy row of Settings. It holds the one
 * Private account switch; the switch itself and its saving live in PrivateAccountRow.
 */
export function AccountPrivacyScreen() {
  return (
    <Screen edges={['top', 'left', 'right']}>
      <View style={{ paddingTop: space.lg, paddingBottom: space.xl }}>
        <BackRow />

        <View style={{ marginTop: space.md }}>
          <AppText role="title" accessibilityRole="header">
            Account privacy
          </AppText>
        </View>

        <View
          style={{
            marginTop: space.lg,
            borderTopWidth: 1,
            borderTopColor: color.divider,
          }}
        >
          <PrivateAccountRow />
        </View>
      </View>
    </Screen>
  );
}

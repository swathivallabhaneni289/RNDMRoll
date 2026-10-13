import { useState } from 'react';
import { View } from 'react-native';
import { AppText } from '@/components/ui/AppText';
import { PrimaryButton } from '@/components/ui/PrimaryButton';
import { Screen } from '@/components/ui/Screen';
import { useSession } from '@/lib/session/store';
import { space } from '@/lib/theme/tokens';

/**
 * Shown over the app when tokens are stored but the server could not be reached at launch. The
 * person is still signed in; Try again runs the launch restore once more.
 */
export function OfflineScreen() {
  const { retryRestore } = useSession();
  const [trying, setTrying] = useState(false);

  async function tryAgain() {
    if (trying) return;
    setTrying(true);
    try {
      await retryRestore();
    } finally {
      setTrying(false);
    }
  }

  return (
    <Screen>
      <View
        style={{ flex: 1, justifyContent: 'center', paddingBottom: space.xl }}
      >
        <AppText role="title" accessibilityRole="header">
          You're offline.
        </AppText>
        <View style={{ marginTop: space.sm }}>
          <AppText role="body" tone="muted">
            Check your connection and try again. You are still logged in.
          </AppText>
        </View>
        <View style={{ marginTop: space.lg }}>
          <PrimaryButton
            label="Try again"
            loading={trying}
            onPress={tryAgain}
          />
        </View>
      </View>
    </Screen>
  );
}

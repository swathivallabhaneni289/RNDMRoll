import { useRouter } from 'expo-router';
import { StatusBar } from 'expo-status-bar';
import { StyleSheet, View, useWindowDimensions } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { AppText } from '@/components/ui/AppText';
import { PrimaryButton } from '@/components/ui/PrimaryButton';
import { PhotoPlaceholder } from '@/components/ui/PhotoPlaceholder';
import {
  PhotoScrim,
  SCRIM_BOTTOM_SAFE_FRACTION,
  SCRIM_TOP_SAFE_FRACTION,
} from '@/components/ui/PhotoScrim';
import { space, type } from '@/lib/theme/tokens';

/**
 * Two lines of Label-role type: the tagline's exact rendered block height,
 * used below to widen the scrim's flat top zone past its declared default
 * whenever a device's own top inset plus this block needs more room.
 */
const TAGLINE_BLOCK_HEIGHT = type.label.lineHeight * 2;

export default function WelcomeScreen() {
  const router = useRouter();
  const insets = useSafeAreaInsets();
  const { height } = useWindowDimensions();

  const topOpaqueFraction = Math.max(
    SCRIM_TOP_SAFE_FRACTION,
    (insets.top + TAGLINE_BLOCK_HEIGHT + space.sm) / height
  );

  return (
    <View style={{ flex: 1 }}>
      <StatusBar style="light" />

      <View style={StyleSheet.absoluteFill}>
        <PhotoPlaceholder variant="cover" />
      </View>

      <PhotoScrim topOpaqueFraction={topOpaqueFraction} />

      <View style={{ position: 'absolute', top: insets.top, left: space.lg }}>
        <AppText role="label" tone="onInk">
          {'one roll.\none real moment.'}
        </AppText>
      </View>

      <View
        style={{
          position: 'absolute',
          left: space.lg,
          right: space.lg,
          top: height * (1 - SCRIM_BOTTOM_SAFE_FRACTION),
          bottom: insets.bottom + space.lg,
          justifyContent: 'flex-end',
        }}
      >
        <View style={{ marginBottom: space.md }}>
          <AppText role="display" tone="onInk">
            {'RNDMRoll'}
          </AppText>
        </View>
        <PrimaryButton label="Get started" variant="onPhoto" onPress={() => router.push('/ritual')} />
      </View>
    </View>
  );
}

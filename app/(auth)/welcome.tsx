import { useRouter } from 'expo-router';
import { StatusBar } from 'expo-status-bar';
import { StyleSheet, View, useWindowDimensions } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { FadeInUp, ImageReveal } from '@/lib/motion/primitives';
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

      <ImageReveal style={StyleSheet.absoluteFill} duration={700}>
        <PhotoPlaceholder variant="cover" />
      </ImageReveal>

      <PhotoScrim topOpaqueFraction={topOpaqueFraction} />

      <FadeInUp delay={60} duration={500} style={{ position: 'absolute', top: insets.top, left: space.lg }}>
        <AppText role="label" tone="onInk">
          {'one roll.\none real moment.'}
        </AppText>
      </FadeInUp>

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
        <FadeInUp delay={160} duration={500} style={{ marginBottom: space.md }}>
          <AppText role="display" tone="onInk">
            {'RNDMRoll'}
          </AppText>
        </FadeInUp>
        <FadeInUp delay={260} duration={500}>
          <PrimaryButton label="Get started" variant="onPhoto" onPress={() => router.push('/ritual')} />
        </FadeInUp>
      </View>
    </View>
  );
}

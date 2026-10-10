import { View } from 'react-native';
import { AppText } from '@/components/ui/AppText';
import { Screen } from '@/components/ui/Screen';
import { space } from '@/lib/theme/tokens';

/**
 * The Board tab. A plain page until Phase 5 fills it: the title and one muted line (decision
 * D-24 of Phase 2; the developer asked for the tab before its content exists). The bar adds the
 * bottom safe-area inset itself, so the screen takes the top and side edges only.
 */
export default function BoardScreen() {
  return (
    <Screen scroll={false} edges={['top', 'left', 'right']}>
      <View style={{ marginTop: space.lg }}>
        <AppText role="title" accessibilityRole="header">
          Board
        </AppText>
        <View style={{ marginTop: space.sm }}>
          <AppText role="label" tone="muted">
            Nothing here yet.
          </AppText>
        </View>
      </View>
    </Screen>
  );
}

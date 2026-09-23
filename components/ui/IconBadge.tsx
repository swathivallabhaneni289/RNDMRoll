import { View } from 'react-native';
import { Ionicons } from '@expo/vector-icons';
import { color, elevation, radius } from '@/lib/theme/tokens';

type IconBadgeProps = {
  name: React.ComponentProps<typeof Ionicons>['name'];
  /** The surface the badge sits ON; the badge fills with the *other* neutral. */
  surface: 'dominant' | 'secondary';
  accessibilityLabel: string;
};

const badgeFillForSurface = {
  dominant: color.secondary,
  secondary: color.dominant,
} as const;

const BADGE_DIAMETER = 40;
const GLYPH_SIZE = 20;

/**
 * The shared 40dp circular icon-badge: `card` elevation's shadow spec,
 * an explicit fixed-width 1dp Ink-inactive border, and a fill that always
 * inverts against the surface it sits on. `accessibilityLabel` is required
 * because every use of this component is icon-only.
 */
export function IconBadge({ name, surface, accessibilityLabel }: IconBadgeProps) {
  return (
    <View
      accessible
      accessibilityLabel={accessibilityLabel}
      style={{
        width: BADGE_DIAMETER,
        height: BADGE_DIAMETER,
        borderRadius: radius.full,
        alignItems: 'center',
        justifyContent: 'center',
        ...elevation.card,
        backgroundColor: badgeFillForSurface[surface],
        borderWidth: 1,
        borderColor: color.inkInactive,
      }}
    >
      <Ionicons name={name} size={GLYPH_SIZE} color={color.ink} />
    </View>
  );
}

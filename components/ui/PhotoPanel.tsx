import { View, ViewStyle } from 'react-native';
import { color, elevation, radius, space } from '@/lib/theme/tokens';
import { PhotoPlaceholder } from '@/components/ui/PhotoPlaceholder';

/**
 * Shared photo-panel surface: `card` elevation's shadow keys plus the
 * `lg` radius, filled with Secondary rather than `card`'s white: these
 * are photo stand-ins, not literal white card surfaces. Not exported;
 * PhotoPanel, PolaroidCard, and SplitPhotoPanels each compose it with
 * their own outer spacing so side margins never compound.
 */
function PanelSurface({ aspectRatio, style }: { aspectRatio?: number; style?: ViewStyle }) {
  return (
    <View
      style={{
        ...elevation.card,
        backgroundColor: color.secondary,
        borderRadius: radius.lg,
        overflow: 'hidden',
        ...(aspectRatio ? { aspectRatio } : null),
        ...style,
      }}
    >
      <PhotoPlaceholder variant="panel" />
    </View>
  );
}

/** One panel, full width inside `space.lg` side margins, portrait-leaning by default. */
export function PhotoPanel({ aspectRatio = 4 / 5 }: { aspectRatio?: number }) {
  return <PanelSurface aspectRatio={aspectRatio} style={{ marginHorizontal: space.lg }} />;
}

/**
 * A square photo area with an additional `space.md` bottom padding
 * strip, simulating a polaroid frame's wider bottom border. `rotation`
 * is a required, caller-supplied number, never randomized, never
 * animated (PROJECT.md bans over-the-top scroll animations).
 */
export function PolaroidCard({ rotation }: { rotation: number }) {
  return (
    <View
      style={{
        ...elevation.card,
        backgroundColor: color.secondary,
        borderRadius: radius.lg,
        overflow: 'hidden',
        paddingBottom: space.md,
        transform: [{ rotate: rotation + 'deg' }],
      }}
    >
      <View style={{ aspectRatio: 1 }}>
        <PhotoPlaceholder variant="panel" />
      </View>
    </View>
  );
}

/** Two panels side by side inside `space.lg` side margins, `space.sm` apart. */
export function SplitPhotoPanels() {
  return (
    <View style={{ flexDirection: 'row', marginHorizontal: space.lg, gap: space.sm }}>
      <View style={{ flex: 1 }}>
        <PanelSurface aspectRatio={4 / 5} />
      </View>
      <View style={{ flex: 1 }}>
        <PanelSurface aspectRatio={4 / 5} />
      </View>
    </View>
  );
}

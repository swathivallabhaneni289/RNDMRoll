import { memo, useEffect } from 'react';
import { View } from 'react-native';
import { useRouter } from 'expo-router';
import { PressScale } from '@/lib/motion/primitives';
import { color, space } from '@/lib/theme/tokens';
import { categoryLabel, LabEntry } from '@/lab/data/entries';
import type { GridRow } from '@/lab/data/diaryLayout';
import { usePhoto } from '@/lab/data/photos';
import { useHeroSource } from '@/lab/motion/hero';
import { grid, photoMetrics } from '@/lab/theme/frost';
import { LabPhoto } from './LabPhoto';
import { LabText } from './LabText';

const GridTile = memo(function GridTile({
  entry,
  width,
  height,
  autoOpen,
}: {
  entry: LabEntry;
  width: number;
  height: number;
  autoOpen: boolean;
}) {
  const router = useRouter();
  const photo = usePhoto(entry);
  const { ref, open, hidden } = useHeroSource(`grid:${entry.id}`, photoMetrics.gridRadius);

  async function handlePress() {
    if (await open()) {
      router.push({ pathname: '/lab/entry/[id]', params: { id: entry.id, from: 'grid' } });
    }
  }

  // Dev aid: opens this tile by itself so the transition can be recorded without touch automation.
  useEffect(() => {
    if (!autoOpen) return;
    const timer = setTimeout(() => {
      void handlePress();
    }, 1600);
    return () => clearTimeout(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [autoOpen]);

  return (
    <PressScale
      onPress={handlePress}
      scaleTo={0.97}
      accessibilityRole="button"
      accessibilityLabel={`${entry.title}, ${categoryLabel(entry.category)}, ${entry.rating} out of 5 stars`}
      testID={`grid-tile-${entry.id}`}
    >
      <View
        ref={ref}
        collapsable={false}
        style={{
          width,
          height,
          borderRadius: photoMetrics.gridRadius,
          overflow: 'hidden',
          backgroundColor: color.secondary,
          opacity: hidden ? 0 : 1,
        }}
      >
        <LabPhoto photo={photo} />
      </View>
    </PressScale>
  );
});

/**
 * One row of the GRID view: either a month heading or up to three photographs. The
 * grid is photographs and whitespace only: no cards, no captions, no ratings on the
 * tiles. Everything else lives one tap away in the detail view.
 */
export const GridRowView = memo(function GridRowView({
  row,
  tileWidth,
  tileHeight,
  autoOpenId,
}: {
  row: GridRow;
  tileWidth: number;
  tileHeight: number;
  autoOpenId?: string;
}) {
  if (row.kind === 'month') {
    return (
      <View style={{ height: row.height, justifyContent: 'flex-end', paddingBottom: space.md }}>
        <LabText role="label" tone="muted" uppercase tracking={1.2}>
          {row.label}
        </LabText>
      </View>
    );
  }

  return (
    <View style={{ height: row.height, flexDirection: 'row', alignItems: 'flex-start', gap: grid.gutter }}>
      {row.entries.map((entry) => (
        <GridTile
          key={entry.id}
          entry={entry}
          width={tileWidth}
          height={tileHeight}
          autoOpen={entry.id === autoOpenId}
        />
      ))}
    </View>
  );
});

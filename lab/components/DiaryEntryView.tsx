import { memo, useEffect } from 'react';
import { View } from 'react-native';
import { useRouter } from 'expo-router';
import { PressScale } from '@/lib/motion/primitives';
import { color, space } from '@/lib/theme/tokens';
import { categoryLabel, formatShort, LabEntry } from '@/lab/data/entries';
import type { DiaryLayout } from '@/lab/data/diaryLayout';
import { usePhoto } from '@/lab/data/photos';
import { useHeroSource } from '@/lab/motion/hero';
import { photoMetrics } from '@/lab/theme/frost';
import { LabPhoto } from './LabPhoto';
import { LabText } from './LabText';
import { StarRating } from './StarRating';

/**
 * One entry in the DIARY view: a large photograph and, beneath it on the plain
 * ground, everything that belongs to the entry. No glass, no card, no border. The
 * archive should read as quiet and timeless, so type does all the work here. The
 * info block has a fixed height so the list can use exact row offsets.
 */
export const DiaryEntryView = memo(function DiaryEntryView({
  entry,
  layout,
  autoOpen,
}: {
  entry: LabEntry;
  layout: DiaryLayout;
  autoOpen: boolean;
}) {
  const router = useRouter();
  const photo = usePhoto(entry);
  const { ref, open, hidden } = useHeroSource(`diary:${entry.id}`, photoMetrics.diaryRadius);

  async function handlePress() {
    if (await open()) {
      router.push({ pathname: '/lab/entry/[id]', params: { id: entry.id, from: 'diary' } });
    }
  }

  // Dev aid: opens this entry by itself so the transition can be recorded without touch automation.
  useEffect(() => {
    if (!autoOpen) return;
    const timer = setTimeout(() => {
      void handlePress();
    }, 1600);
    return () => clearTimeout(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [autoOpen]);

  return (
    <View style={{ height: layout.itemHeight }}>
      <PressScale
        onPress={handlePress}
        scaleTo={0.985}
        accessibilityRole="button"
        accessibilityLabel={`${entry.title}. ${categoryLabel(entry.category)}. ${entry.rating} out of 5 stars. ${entry.reaction}`}
        accessibilityHint="Opens the entry"
        testID={`diary-entry-${entry.id}`}
      >
        <View
          ref={ref}
          collapsable={false}
          style={{
            width: layout.photoWidth,
            height: layout.photoHeight,
            borderRadius: photoMetrics.diaryRadius,
            overflow: 'hidden',
            backgroundColor: color.secondary,
            opacity: hidden ? 0 : 1,
          }}
        >
          <LabPhoto photo={photo} />
        </View>
      </PressScale>

      <View style={{ height: layout.infoHeight, paddingTop: space.md, overflow: 'hidden' }}>
        <View style={{ flexDirection: 'row', justifyContent: 'space-between', alignItems: 'baseline' }}>
          <LabText role="label" uppercase tracking={1.2}>
            {categoryLabel(entry.category)}
          </LabText>
          <LabText role="label" tone="muted">
            {formatShort(entry.date)}
          </LabText>
        </View>
        <LabText role="heading" numberOfLines={1} style={{ marginTop: space.sm }}>
          {entry.title}
        </LabText>
        <View style={{ marginTop: space.xs }}>
          <StarRating value={entry.rating} size={16} />
        </View>
        <LabText role="body" numberOfLines={2} style={{ marginTop: space.sm }}>
          {entry.reaction}
        </LabText>
        <LabText role="label" tone="muted" style={{ marginTop: space.sm }}>
          {entry.rolledAt}
        </LabText>
      </View>
    </View>
  );
});

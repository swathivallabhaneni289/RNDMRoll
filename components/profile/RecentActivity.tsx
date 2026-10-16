import { useState } from 'react';
import { Pressable, View } from 'react-native';
import { Image } from 'expo-image';
import { Ionicons } from '@expo/vector-icons';
import { AppText } from '@/components/ui/AppText';
import { SectionLabel } from '@/components/profile/SectionLabel';
import { recentLabel, recentSpoken } from '@/components/profile/summaryText';
import type { RecentEntry } from '@/lib/api/summary';
import { color, radius, space } from '@/lib/theme/tokens';

const THUMB = 48;

/** The 48dp square photo of a row: a plain `secondary` panel with a small glyph when it cannot load. */
function Thumb({ uri }: { uri?: string | null }) {
  const [failed, setFailed] = useState(false);
  const showPhoto = typeof uri === 'string' && uri !== '' && !failed;
  return (
    <View
      accessibilityElementsHidden
      importantForAccessibility="no-hide-descendants"
      style={{
        width: THUMB,
        height: THUMB,
        borderRadius: radius.md,
        backgroundColor: color.secondary,
        overflow: 'hidden',
        alignItems: 'center',
        justifyContent: 'center',
      }}
    >
      {showPhoto ? (
        <Image
          source={{ uri }}
          style={{ width: THUMB, height: THUMB }}
          contentFit="cover"
          onError={() => setFailed(true)}
        />
      ) : (
        <Ionicons name="image-outline" size={20} color={color.muted} />
      )}
    </View>
  );
}

function Row({ entry, onPress }: { entry: RecentEntry; onPress?: () => void }) {
  const inner = (
    <View style={{ flexDirection: 'row', alignItems: 'center', paddingVertical: space.sm }}>
      <Thumb uri={entry.photo_url} />
      <View style={{ flex: 1, marginLeft: space.md }}>
        <AppText role="note" numberOfLines={1} ellipsizeMode="tail">
          {entry.title}
        </AppText>
        <View style={{ marginTop: space.xs }}>
          <SectionLabel spacing={1.2}>{recentLabel(entry.category.name, entry.local_date)}</SectionLabel>
        </View>
      </View>
    </View>
  );
  const rule = { borderBottomWidth: 1, borderBottomColor: color.divider } as const;
  const spoken = recentSpoken(entry.title, entry.category.name, entry.local_date);

  if (!onPress) {
    return (
      <View accessible accessibilityLabel={spoken} style={rule}>
        {inner}
      </View>
    );
  }
  return (
    <Pressable
      onPress={onPress}
      accessibilityRole="button"
      accessibilityLabel={spoken}
      accessibilityHint="Opens the entry."
      style={({ pressed }) => ({ ...rule, backgroundColor: pressed ? color.secondary : 'transparent' })}
    >
      {inner}
    </Pressable>
  );
}

/**
 * "RECENT ACTIVITY" (04-UI-SPEC 4.4): the label (no 'See diary' link: the Diary has its tab), then the
 * person's last posts (at most three, newest first): a small photo, the title in the light serif
 * and "MEAL \u00B7 OCT 8" under it, a hairline under each row. With none, one quiet line.
 *
 * A row becomes a button only when `onOpenEntry` is given (the Profile page gives it: the row
 * opens Phase 3's entry page inside the Profile stack).
 */
export function RecentActivity({
  recent,
  onOpenEntry,
}: {
  recent: RecentEntry[];
  onOpenEntry?: (entryId: string) => void;
}) {
  return (
    <View>
      <SectionLabel header>Recent activity</SectionLabel>
      {recent.length === 0 ? (
        <View style={{ marginTop: space.sm }}>
          <AppText role="label" tone="muted">
            Nothing here yet.
          </AppText>
        </View>
      ) : (
        <View>
          {recent.slice(0, 3).map((entry) => (
            <Row key={entry.id} entry={entry} onPress={onOpenEntry ? () => onOpenEntry(entry.id) : undefined} />
          ))}
        </View>
      )}
    </View>
  );
}

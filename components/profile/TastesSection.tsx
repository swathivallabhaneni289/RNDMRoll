import { View } from 'react-native';
import { AppText } from '@/components/ui/AppText';
import { Chip } from '@/components/ui/Chip';
import { SectionLabel } from '@/components/profile/SectionLabel';
import { averageText, tasteSpoken } from '@/components/profile/summaryText';
import type { Taste } from '@/lib/api/summary';
import { space } from '@/lib/theme/tokens';

const COLUMNS = 3;

/**
 * "YOUR TASTES \u00B7 AVG RATING" (04-UI-SPEC 4.3): one rounded box per category the person has
 * posted in, the category at the left and the average at the right, three to a row with 8dp gaps,
 * in the order the server sent (the wheel's order, then removed categories). With none, one quiet
 * line stands in their place.
 *
 * A box becomes a button only when `onSelect` is given. It is left out until the Diary can open on
 * one category (the Diary's filter is the next piece), so nothing on this page is a dead control.
 */
export function TastesSection({
  tastes,
  onSelect,
}: {
  tastes: Taste[];
  onSelect?: (taste: Taste) => void;
}) {
  const rows: Taste[][] = [];
  for (let i = 0; i < tastes.length; i += COLUMNS) rows.push(tastes.slice(i, i + COLUMNS));

  return (
    <View>
      <SectionLabel header>{'Your tastes \u00B7 avg rating'}</SectionLabel>
      <View style={{ marginTop: space.sm }}>
        {tastes.length === 0 ? (
          <AppText role="label" tone="muted">
            Your tastes show here after your first posts.
          </AppText>
        ) : (
          rows.map((row, rowIndex) => (
            <View key={rowIndex} style={{ flexDirection: 'row', marginTop: rowIndex === 0 ? 0 : space.sm }}>
              {Array.from({ length: COLUMNS }, (_, column) => {
                const taste = row[column];
                return (
                  <View key={column} style={{ flex: 1, marginLeft: column === 0 ? 0 : space.sm }}>
                    {taste ? (
                      <Chip
                        label={taste.category.name}
                        value={averageText(taste.average)}
                        tone="outline"
                        height={40}
                        onPress={onSelect ? () => onSelect(taste) : undefined}
                        accessibilityLabel={tasteSpoken(taste.category.name, taste.average, taste.count)}
                        accessibilityHint={onSelect ? 'Opens your diary.' : undefined}
                      />
                    ) : null}
                  </View>
                );
              })}
            </View>
          ))
        )}
      </View>
    </View>
  );
}

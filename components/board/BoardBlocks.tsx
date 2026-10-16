import { Text, View } from 'react-native';
import { NameChip, capsText } from '@/components/board/PostCard';
import { AppText } from '@/components/ui/AppText';
import { PrimaryButton } from '@/components/ui/PrimaryButton';
import { TextButton } from '@/components/ui/TextButton';
import type { BoardLocked, DayLabel } from '@/lib/api/board';
import { LOCKED_BODY, NO_FOLLOWS_LINE, lockedButton, lockedFootnote, postedLine, skippedLine } from '@/lib/board/text';
import { color, elevation, radius, space, type } from '@/lib/theme/tokens';

const TILE_ASPECT = 7 / 8;
const BAR_HEIGHT = 8;

/** One grey tile behind the locked card: the photo area with a name chip, and two grey bars. */
function Tile({ name, width }: { name: string; width: number }) {
  return (
    <View style={{ width }}>
      <View style={{ width, height: Math.round(width / TILE_ASPECT), backgroundColor: color.secondary, padding: space.sm }}>
        <NameChip person={null} label={name} />
      </View>
      <View style={{ marginTop: space.sm, width: width * 0.6, height: BAR_HEIGHT, borderRadius: radius.sm, backgroundColor: color.divider }} />
      <View style={{ marginTop: space.xs, width: width * 0.35, height: BAR_HEIGHT, borderRadius: radius.sm, backgroundColor: color.divider }} />
    </View>
  );
}

function TileRow({ names, width }: { names: string[]; width: number }) {
  return (
    <View style={{ flexDirection: 'row', justifyContent: 'space-between' }}>
      {names.map((name, index) => (
        <Tile key={`${name}-${index}`} name={name} width={width} />
      ))}
      {names.length === 1 ? <View style={{ width }} /> : null}
    </View>
  );
}

/**
 * Locked (05-UI-SPEC 3.6, the developer's picture 08): up to four grey tiles with the first names of
 * people you follow who posted, and the card over them. The tiles never show a photo, title or
 * stars. The button follows the server's `action`: log opens the entry screen, spin goes to the
 * Spin page, wait draws no button.
 */
export function LockedBlock({
  locked,
  dayLabel,
  columnWidth,
  onLog,
  onSpin,
}: {
  locked: BoardLocked;
  dayLabel: DayLabel;
  columnWidth: number;
  onLog: () => void;
  onSpin: () => void;
}) {
  const names = locked.names.slice(0, 4);
  const first = names.slice(0, 2);
  const second = names.slice(2, 4);
  const tileHeight = Math.round(columnWidth / TILE_ASPECT);
  const button = lockedButton(locked.action, locked.category);
  const footnote = lockedFootnote(locked.action, dayLabel);
  const heading = postedLine(locked.posted_count, dayLabel);

  const card = (
    <View
      accessible={false}
      style={{
        ...elevation.card,
        borderRadius: radius.md,
        borderWidth: 1,
        borderColor: color.ink,
        padding: space.lg,
        zIndex: 1,
      }}
    >
      <View accessible accessibilityLabel={`Locked. ${heading} ${LOCKED_BODY}`}>
        <Text style={{ ...capsText, color: color.muted }}>Locked</Text>
        <View style={{ marginTop: space.sm }}>
          <AppText role="note">{heading}</AppText>
        </View>
        <View style={{ marginTop: space.sm }}>
          <AppText role="body" tone="muted">
            {LOCKED_BODY}
          </AppText>
        </View>
      </View>
      {footnote !== null ? (
        <View style={{ marginTop: space.sm }}>
          <AppText role="label" tone="muted">
            {footnote}
          </AppText>
        </View>
      ) : null}
      {button !== null ? (
        <View style={{ marginTop: space.md }}>
          <PrimaryButton label={button} onPress={locked.action === 'log' ? onLog : onSpin} />
        </View>
      ) : null}
    </View>
  );

  if (first.length === 0) return <View style={{ paddingTop: space.sm }}>{card}</View>;

  // The card sits over the lower half of the first row and the top of the second.
  const overlap = Math.round(tileHeight * 0.5);
  return (
    <View style={{ paddingTop: space.sm }}>
      <View accessibilityElementsHidden importantForAccessibility="no-hide-descendants">
        <TileRow names={first} width={columnWidth} />
      </View>
      <View style={{ marginTop: -(overlap + BAR_HEIGHT * 2 + space.sm + space.xs), zIndex: 1 }}>{card}</View>
      {second.length > 0 ? (
        <View
          accessibilityElementsHidden
          importantForAccessibility="no-hide-descendants"
          style={{ marginTop: -Math.round(tileHeight * 0.35), zIndex: 0 }}
        >
          <TileRow names={second} width={columnWidth} />
        </View>
      ) : null}
    </View>
  );
}

/** Skipped (05-UI-SPEC 3.7): the one sentence and 'Go to Spin'. No grid for the day in play. */
export function SkippedBlock({ dayLabel, onGoToSpin }: { dayLabel: DayLabel; onGoToSpin: () => void }) {
  return (
    <View style={{ paddingTop: space.lg, alignItems: 'center' }}>
      <View style={{ alignSelf: 'stretch' }}>
        <Text
          accessibilityLiveRegion="polite"
          style={{
            fontFamily: type.note.fontFamily,
            fontSize: type.note.fontSize,
            lineHeight: type.note.lineHeight,
            color: color.ink,
            textAlign: 'center',
          }}
        >
          {skippedLine(dayLabel)}
        </Text>
      </View>
      <View style={{ marginTop: space.md }}>
        <TextButton label="Go to Spin" onPress={onGoToSpin} />
      </View>
    </View>
  );
}

/** No follows yet: one plain prompt and 'Find people'. */
export function NoFollowsBlock({ onFindPeople }: { onFindPeople: () => void }) {
  return (
    <View style={{ paddingTop: space.lg }}>
      <AppText role="body">{NO_FOLLOWS_LINE}</AppText>
      <View style={{ marginTop: space.md }}>
        <PrimaryButton label="Find people" onPress={onFindPeople} />
      </View>
    </View>
  );
}

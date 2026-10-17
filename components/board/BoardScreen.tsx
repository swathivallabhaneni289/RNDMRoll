import { FlatList, Pressable, RefreshControl, Text, useWindowDimensions, View } from 'react-native';
import { Ionicons } from '@expo/vector-icons';
import { SafeAreaView } from 'react-native-safe-area-context';
import { LockedBlock, NoFollowsBlock, SkippedBlock } from '@/components/board/BoardBlocks';
import { PostCard, capsText } from '@/components/board/PostCard';
import { useBoard } from '@/components/board/useBoard';
import { STATUS_MESSAGES, StatusMessage } from '@/components/today/StatusMessage';
import { AppText } from '@/components/ui/AppText';
import { TextButton } from '@/components/ui/TextButton';
import type { BoardDay, Card } from '@/lib/api/board';
import { overline } from '@/lib/board/text';
import { color, minTouchTarget, space } from '@/lib/theme/tokens';
import { dateLine } from '@/lib/wheel/format';
import { useOpenPerson } from '@/lib/people/openPerson';

type Row =
  | { kind: 'heading'; key: string; text: string }
  | { kind: 'pair'; key: string; cards: [Card, Card | null] };

/**
 * The server's days as one flat list: a heading for every day but the day in play (the page's
 * overline names it), then the cards two to a row in the order sent.
 */
function toRows(days: BoardDay[]): Row[] {
  const rows: Row[] = [];
  for (const day of days) {
    if (day.label !== 'today') {
      rows.push({
        kind: 'heading',
        key: `h-${day.local_date}`,
        text: day.label === 'yesterday' ? 'Yesterday' : dateLine(day.local_date),
      });
    }
    for (let i = 0; i < day.cards.length; i += 2) {
      rows.push({ kind: 'pair', key: `p-${day.local_date}-${day.cards[i].entry_id}`, cards: [day.cards[i], day.cards[i + 1] ?? null] });
    }
  }
  return rows;
}

/** A quiet centred line: loading, and the list's footer. */
function QuietLine({ children }: { children: string }) {
  return (
    <View style={{ paddingVertical: space.md, alignItems: 'center' }}>
      <AppText role="label" tone="muted" accessibilityLiveRegion="polite">
        {children}
      </AppText>
    </View>
  );
}

export type BoardScreenProps = {
  onFindPeople: () => void;
  onOpenCard: (card: Card) => void;
  onLog: () => void;
  onSpin: () => void;
};

/**
 * The Board tab (05-UI-SPEC section 3 with the follow wording of 05-FOLLOW-OVERRIDES, the
 * developer's pictures 08 and 09). A fixed header (the overline, 'Board' and a search icon that opens
 * Search; the bell lives on the Spin page only, 2026-10-16), then what the server says: no follows (a prompt and 'Find people'), locked
 * (the locked card over grey tiles), skipped, or unlocked (the two-column grid). Under a lock or a
 * skip the earlier days the person posted still follow. Plain light cards, nothing blurs or fades.
 * The list pages as the end comes into view, pulls to refresh and reloads on every focus.
 */
export function BoardScreen({ onFindPeople, onOpenCard, onLog, onSpin }: BoardScreenProps) {
  const openPerson = useOpenPerson();
  const { width } = useWindowDimensions();
  const load = useBoard();
  const board = load.board;
  const columnWidth = Math.floor((width - 2 * space.lg - space.md) / 2);

  const header = (
    <View style={{ paddingHorizontal: space.lg, paddingTop: space.sm, flexDirection: 'row', alignItems: 'flex-end' }}>
      <View style={{ flex: 1 }} accessible accessibilityRole="header" accessibilityLabel={board ? `${overline(board.day_label)}, Board` : 'Board'}>
        <Text style={{ ...capsText, color: color.muted, minHeight: capsText.lineHeight }}>
          {board ? overline(board.day_label) : ''}
        </Text>
        <View style={{ marginTop: space.xs }}>
          <AppText role="title">Board</AppText>
        </View>
      </View>
      <View style={{ flexDirection: 'row', alignItems: 'center', marginBottom: space.xs }}>
        <Pressable
          onPress={onFindPeople}
          accessibilityRole="button"
          accessibilityLabel="Search people"
          hitSlop={4}
          style={{ width: minTouchTarget, height: minTouchTarget, alignItems: 'flex-end', justifyContent: 'center' }}
        >
          <Ionicons name="search-outline" size={22} color={color.ink} />
        </Pressable>
      </View>
    </View>
  );

  function renderRow({ item }: { item: Row }) {
    if (item.kind === 'heading') {
      return (
        <View style={{ paddingHorizontal: space.lg, marginTop: space.sm, marginBottom: space.sm + space.xs }}>
          <Text accessibilityRole="header" style={{ ...capsText, color: color.muted }}>
            {item.text}
          </Text>
        </View>
      );
    }
    const [a, b] = item.cards;
    return (
      <View style={{ paddingHorizontal: space.lg, marginBottom: space.lg, flexDirection: 'row', justifyContent: 'space-between' }}>
        <PostCard card={a} width={columnWidth} onPress={() => onOpenCard(a)} onOpenPerson={openPerson} />
        {b ? <PostCard card={b} width={columnWidth} onPress={() => onOpenCard(b)} onOpenPerson={openPerson} /> : <View style={{ width: columnWidth }} />}
      </View>
    );
  }

  let body;
  if (load.status === 'loading' || board === null) {
    body =
      load.status === 'offline' ? (
        <View style={{ paddingHorizontal: space.lg, marginTop: space.xl }}>
          <StatusMessage kind={load.failure} onRetry={load.retry} />
        </View>
      ) : (
        <View style={{ marginTop: space.xl }}>
          <QuietLine>Loading the Board.</QuietLine>
        </View>
      );
  } else if (load.status === 'offline') {
    body = (
      <View style={{ paddingHorizontal: space.lg, marginTop: space.xl }}>
        <StatusMessage kind={load.failure} onRetry={load.retry} />
      </View>
    );
  } else {
    const listHeader =
      board.state === 'locked' && board.locked ? (
        <View style={{ paddingHorizontal: space.lg, marginBottom: board.days.length > 0 ? space.md : 0 }}>
          <LockedBlock locked={board.locked} dayLabel={board.day_label} columnWidth={columnWidth} onLog={onLog} onSpin={onSpin} />
        </View>
      ) : board.state === 'skipped' ? (
        <View style={{ paddingHorizontal: space.lg, marginBottom: space.md }}>
          <SkippedBlock dayLabel={board.day_label} onGoToSpin={onSpin} />
        </View>
      ) : board.state === 'no_follows' ? (
        <View style={{ paddingHorizontal: space.lg }}>
          <NoFollowsBlock onFindPeople={onFindPeople} />
        </View>
      ) : null;

    const footer = load.loadingMore ? (
      <QuietLine>Loading more.</QuietLine>
    ) : load.moreFailed ? (
      <View style={{ paddingHorizontal: space.lg, paddingVertical: space.md, alignItems: 'center' }}>
        <AppText role="label" accessibilityLiveRegion="polite">
          {STATUS_MESSAGES.network}
        </AppText>
        <View style={{ marginTop: space.sm }}>
          <TextButton label="Try again" onPress={load.loadMore} />
        </View>
      </View>
    ) : null;

    const empty =
      board.state === 'unlocked' ? (
        <View style={{ marginTop: space.xl }}>
          <QuietLine>Nothing here yet.</QuietLine>
        </View>
      ) : null;

    body = (
      <FlatList
        style={{ flex: 1 }}
        data={board.state === 'no_follows' ? [] : toRows(board.days)}
        keyExtractor={(row) => row.key}
        renderItem={renderRow}
        ListHeaderComponent={listHeader}
        ListEmptyComponent={empty}
        ListFooterComponent={footer}
        onEndReached={load.loadMore}
        onEndReachedThreshold={0.5}
        refreshControl={<RefreshControl refreshing={load.refreshing} onRefresh={load.refresh} tintColor={color.muted} />}
        contentContainerStyle={{ paddingTop: space.md, paddingBottom: space.xl }}
      />
    );
  }

  return (
    <SafeAreaView edges={['top', 'left', 'right']} style={{ flex: 1, backgroundColor: color.dominant }}>
      {header}
      {body}
    </SafeAreaView>
  );
}

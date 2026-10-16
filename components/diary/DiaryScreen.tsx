import { useState } from 'react';
import { Alert, FlatList, RefreshControl, View } from 'react-native';
import { useRouter } from 'expo-router';
import { SafeAreaView } from 'react-native-safe-area-context';
import { DiaryEntryRow, DiarySkippedRow } from '@/components/diary/DiaryRow';
import { rowKey, useDiary } from '@/components/diary/useDiary';
import { STATUS_MESSAGES, StatusMessage } from '@/components/today/StatusMessage';
import { AppText } from '@/components/ui/AppText';
import { MenuSheet, type MenuItem } from '@/components/ui/MenuSheet';
import { TextButton } from '@/components/ui/TextButton';
import { ApiError } from '@/lib/api/client';
import type { DiaryItem } from '@/lib/api/diary';
import { archiveEntry, deleteEntry, entryErrorMessage, updateEntry, type Entry } from '@/lib/api/entries';
import type { Today } from '@/lib/api/types';
import { useSession } from '@/lib/session/store';
import { color, space } from '@/lib/theme/tokens';
import { applyToday } from '@/lib/wheel/todayStore';
import { refetchToday } from '@/lib/wheel/useToday';

/** A quiet centred line under the header: loading, empty, and the list's footer. */
function QuietLine({ children }: { children: string }) {
  return (
    <View style={{ paddingHorizontal: space.lg, paddingVertical: space.md, alignItems: 'center' }}>
      <AppText role="label" tone="muted" accessibilityLiveRegion="polite">
        {children}
      </AppText>
    </View>
  );
}

/**
 * The Diary tab (04-UI-SPEC section 3, the developer's picture 07): the heading and a hairline stay
 * put, the list scrolls under them. Posts are rows with the cover, date and category, title and
 * stars, and three dots that open the same menu as the entry page (Edit, Archive, commenting,
 * Delete); skipped days are quiet rows. There are no category chips (the developer removed them).
 * The list pages as the end comes into view, pulls to refresh and reloads on every focus. A tap
 * on a post opens the entry page inside this tab's stack, so Back reads 'Diary'. Nothing animates.
 */
export function DiaryScreen() {
  const router = useRouter();
  const { user } = useSession();
  const userId = user?.id ?? null;
  const diary = useDiary(null);
  const [menuFor, setMenuFor] = useState<Entry | null>(null);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState('');

  function open(id: string) {
    router.push({ pathname: '/(app)/diary/entry', params: { id, from: 'diary' } });
  }

  /** Runs one action from a row's menu, then reads the list and Today again. */
  async function act(work: () => Promise<{ today?: Today } | unknown>) {
    if (busy) return;
    setBusy(true);
    setMessage('');
    const mark = performance.now();
    try {
      const answer = (await work()) as { today?: Today } | undefined;
      if (userId !== null) {
        if (answer?.today) applyToday(userId, answer.today, mark);
        void refetchToday(userId, 'manual');
      }
    } catch (err) {
      if (!(err instanceof ApiError && err.code === 'not_found')) setMessage(entryErrorMessage(err).text);
    } finally {
      setBusy(false);
      diary.reload();
    }
  }

  function askDelete(entry: Entry) {
    Alert.alert(
      'Delete this entry?',
      "It will be gone for good. The day still counts as posted, so you can't post again for this day.",
      [
        { text: 'Cancel', style: 'cancel' },
        { text: 'Delete', onPress: () => void act(() => deleteEntry(entry.id)) },
      ],
      { cancelable: true }
    );
  }

  /** Close the menu first, so an alert or a page never opens under its dim cover. */
  function fromMenu(action: () => void) {
    return () => {
      setMenuFor(null);
      setTimeout(action, 0);
    };
  }

  const menuItems: MenuItem[] =
    menuFor === null
      ? []
      : [
          {
            label: 'Edit',
            onPress: fromMenu(() => router.push({ pathname: '/(app)/diary/edit-entry', params: { id: menuFor.id } })),
          },
          { label: 'Archive', onPress: fromMenu(() => void act(() => archiveEntry(menuFor.id))) },
          {
            label: menuFor.comments_off ? 'Turn on commenting' : 'Turn off commenting',
            onPress: fromMenu(() => void act(() => updateEntry(menuFor.id, { comments_off: !menuFor.comments_off }))),
          },
          { label: 'Delete', onPress: fromMenu(() => askDelete(menuFor)) },
        ];

  function renderItem({ item }: { item: DiaryItem }) {
    if (item.kind === 'entry') {
      return (
        <DiaryEntryRow
          entry={item.entry}
          onPress={() => open(item.entry.id)}
          onMore={() => !busy && setMenuFor(item.entry)}
        />
      );
    }
    return <DiarySkippedRow localDate={item.local_date} categoryName={item.category.name} />;
  }

  const empty = 'Nothing here yet.';

  const footer = diary.loadingMore ? (
    <QuietLine>Loading more.</QuietLine>
  ) : diary.moreFailed ? (
    <View style={{ paddingHorizontal: space.lg, paddingVertical: space.md, alignItems: 'center' }}>
      <AppText role="label" tone="default" accessibilityLiveRegion="polite">
        {STATUS_MESSAGES.network}
      </AppText>
      <View style={{ marginTop: space.sm }}>
        <TextButton label="Try again" onPress={diary.retryMore} />
      </View>
    </View>
  ) : null;

  return (
    <SafeAreaView edges={['top', 'left', 'right']} style={{ flex: 1, backgroundColor: color.dominant }}>
      <View style={{ paddingHorizontal: space.lg, marginTop: space.lg }}>
        <AppText role="title" accessibilityRole="header">
          Diary
        </AppText>
      </View>
      {message !== '' ? (
        <View style={{ paddingHorizontal: space.lg, marginTop: space.sm }} accessibilityLiveRegion="polite">
          <AppText role="label">{message}</AppText>
        </View>
      ) : null}
      <View style={{ marginTop: space.md, height: 1, backgroundColor: color.divider }} />

      {diary.status === 'loading' ? (
        <View style={{ marginTop: space.xl }}>
          <QuietLine>Loading your diary.</QuietLine>
        </View>
      ) : diary.status === 'offline' ? (
        <View style={{ paddingHorizontal: space.lg, marginTop: space.xl }}>
          <StatusMessage kind={diary.failure} onRetry={diary.retry} />
        </View>
      ) : (
        <FlatList
          style={{ flex: 1 }}
          data={diary.rows}
          keyExtractor={rowKey}
          renderItem={renderItem}
          onEndReached={diary.loadMore}
          onEndReachedThreshold={0.5}
          refreshControl={
            <RefreshControl refreshing={diary.refreshing} onRefresh={diary.refresh} tintColor={color.muted} />
          }
          ListEmptyComponent={
            <View style={{ marginTop: space.xl }}>
              <QuietLine>{empty}</QuietLine>
            </View>
          }
          ListFooterComponent={footer}
          contentContainerStyle={{ paddingBottom: space.xl }}
        />
      )}
      <MenuSheet visible={menuFor !== null} items={menuItems} onClose={() => setMenuFor(null)} />
    </SafeAreaView>
  );
}

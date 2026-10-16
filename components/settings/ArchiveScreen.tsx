import { useCallback, useState } from 'react';
import { Pressable, ScrollView, View } from 'react-native';
import { Ionicons } from '@expo/vector-icons';
import { Image } from 'expo-image';
import { useFocusEffect } from 'expo-router';
import { AppText } from '@/components/ui/AppText';
import { Screen } from '@/components/ui/Screen';
import { BackRow } from '@/components/settings/BackRow';
import { getArchive, unarchiveEntry, entryErrorMessage, type Entry } from '@/lib/api/entries';
import { useSession } from '@/lib/session/store';
import { datePrefix } from '@/components/diary/diaryText';
import { color, minTouchTarget, radius, space } from '@/lib/theme/tokens';
import { applyToday } from '@/lib/wheel/todayStore';
import { refetchToday } from '@/lib/wheel/useToday';

const THUMB = 64;

/**
 * Settings > Archive: the posts the person hid, newest day first, each with a restore arrow at the right. Restoring a
 * post makes no notification and tells nobody; it simply shows again on the profile and Home.
 */
export function ArchiveScreen() {
  const { user } = useSession();
  const userId = user?.id ?? null;
  const [entries, setEntries] = useState<Entry[] | null>(null);
  const [failed, setFailed] = useState(false);
  const [busyId, setBusyId] = useState<string | null>(null);
  const [message, setMessage] = useState('');

  const load = useCallback(async () => {
    try {
      setEntries(await getArchive());
      setFailed(false);
    } catch {
      setFailed(true);
    }
  }, []);

  useFocusEffect(
    useCallback(() => {
      void load();
    }, [load])
  );

  async function putBack(entry: Entry) {
    if (busyId !== null) return;
    setBusyId(entry.id);
    setMessage('');
    const mark = performance.now();
    try {
      const answer = await unarchiveEntry(entry.id);
      if (userId !== null) {
        applyToday(userId, answer.today, mark);
        void refetchToday(userId, 'manual');
      }
      setEntries((list) => (list === null ? list : list.filter((item) => item.id !== entry.id)));
    } catch (err) {
      setMessage(entryErrorMessage(err).text);
    } finally {
      setBusyId(null);
    }
  }

  return (
    <Screen edges={['top', 'left', 'right']}>
      <View style={{ paddingTop: space.lg, paddingBottom: space.xl, flex: 1 }}>
        <BackRow />
        <View style={{ marginTop: space.md }}>
          <AppText role="title" accessibilityRole="header">
            Archive
          </AppText>
        </View>
        <View style={{ marginTop: space.sm }}>
          <AppText role="body" tone="muted">
            Only you can see these.
          </AppText>
        </View>
        {message !== '' ? (
          <View accessibilityLiveRegion="polite" style={{ marginTop: space.sm }}>
            <AppText role="label" tone="default">
              {message}
            </AppText>
          </View>
        ) : null}

        <ScrollView style={{ marginTop: space.lg }} showsVerticalScrollIndicator={false}>
          {entries === null ? (
            <AppText role="label" tone="muted">
              {failed ? "Couldn't load your archive." : 'Loading your archive.'}
            </AppText>
          ) : entries.length === 0 ? (
            <AppText role="body">Nothing is archived.</AppText>
          ) : (
            entries.map((entry) => (
              <View
                key={entry.id}
                style={{
                  flexDirection: 'row',
                  alignItems: 'center',
                  paddingVertical: space.sm,
                  borderBottomWidth: 1,
                  borderBottomColor: color.divider,
                }}
              >
                <View
                  style={{
                    width: THUMB,
                    height: THUMB,
                    borderRadius: radius.md,
                    overflow: 'hidden',
                    backgroundColor: color.secondary,
                  }}
                >
                  {entry.photo_url !== '' ? (
                    <Image source={{ uri: entry.photo_url }} style={{ width: '100%', height: '100%' }} contentFit="cover" />
                  ) : null}
                </View>
                <View style={{ flex: 1, marginHorizontal: space.md }}>
                  <AppText role="label" tone="muted" numberOfLines={1}>
                    {`${datePrefix(entry.local_date)}${entry.category.name}`}
                  </AppText>
                  <AppText role="note" numberOfLines={1} ellipsizeMode="tail">
                    {entry.title}
                  </AppText>
                </View>
                <Pressable
                  onPress={() => void putBack(entry)}
                  disabled={busyId !== null}
                  accessibilityRole="button"
                  accessibilityLabel={`Restore ${entry.title}`}
                  accessibilityHint="Shows it on your profile again. Nobody is told."
                  style={{ width: minTouchTarget, height: minTouchTarget, alignItems: 'flex-end', justifyContent: 'center' }}
                >
                  <Ionicons name="arrow-undo-outline" size={22} color={busyId !== null ? color.muted : color.ink} />
                </Pressable>
              </View>
            ))
          )}
        </ScrollView>
      </View>
    </Screen>
  );
}

import { useRef, useState } from 'react';
import { Alert } from 'react-native';
import { useRouter } from 'expo-router';
import { editEntryPath, type EntryFrom } from '@/components/entry/entryFrom';
import type { MenuItem } from '@/components/ui/MenuSheet';
import { ApiError } from '@/lib/api/client';
import { archiveEntry, deleteEntry, entryErrorMessage, updateEntry, type Entry } from '@/lib/api/entries';
import { useSession } from '@/lib/session/store';
import { applyToday } from '@/lib/wheel/todayStore';
import { refetchToday } from '@/lib/wheel/useToday';

/**
 * The three-dots menu of one of your own posts in a list (the Profile posts list): Edit, Archive,
 * commenting on or off, and Delete, with the same wording and alert as the entry page. `onChanged`
 * runs after anything that changes the list (an archive, a delete, a commenting switch), so the
 * list can read itself again. One action at a time; a failure leaves one plain line in `message`.
 */
export function useEntryMenu(from: EntryFrom, onChanged: () => void) {
  const router = useRouter();
  const { user } = useSession();
  const userId = user?.id ?? null;
  const [target, setTarget] = useState<Entry | null>(null);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  const busyRef = useRef(false);

  async function run(work: () => Promise<void>) {
    if (busyRef.current) return;
    busyRef.current = true;
    setBusy(true);
    setMessage(null);
    try {
      await work();
      onChanged();
    } catch (err) {
      if (err instanceof ApiError && err.code === 'not_found') onChanged();
      else setMessage(entryErrorMessage(err).text);
    } finally {
      busyRef.current = false;
      setBusy(false);
    }
  }

  /** Run after the menu has closed, so an alert or a page never opens under its dim cover. */
  function afterClose(action: () => void) {
    return () => {
      setTarget(null);
      setTimeout(action, 0);
    };
  }

  function askDelete(entry: Entry) {
    Alert.alert(
      'Delete this entry?',
      "It will be gone for good. The day still counts as posted, so you can't post again for this day.",
      [
        { text: 'Cancel', style: 'cancel' },
        {
          text: 'Delete',
          onPress: () =>
            void run(async () => {
              const mark = performance.now();
              const answer = await deleteEntry(entry.id);
              if (userId !== null) {
                applyToday(userId, answer.today, mark);
                void refetchToday(userId, 'manual');
              }
            }),
        },
      ],
      { cancelable: true }
    );
  }

  const items: MenuItem[] =
    target === null
      ? []
      : [
          {
            label: 'Edit',
            onPress: afterClose(() => router.push({ pathname: editEntryPath(from), params: { id: target.id } })),
          },
          {
            label: 'Archive',
            onPress: afterClose(
              () =>
                void run(async () => {
                  const mark = performance.now();
                  const answer = await archiveEntry(target.id);
                  if (userId !== null) {
                    applyToday(userId, answer.today, mark);
                    void refetchToday(userId, 'manual');
                  }
                })
            ),
          },
          {
            label: target.comments_off ? 'Turn on commenting' : 'Turn off commenting',
            onPress: afterClose(
              () =>
                void run(async () => {
                  await updateEntry(target.id, { comments_off: !target.comments_off });
                })
            ),
          },
          { label: 'Delete', onPress: afterClose(() => askDelete(target)) },
        ];

  return {
    /** Opens the menu for one post. */
    open: (entry: Entry) => {
      if (!busyRef.current) setTarget(entry);
    },
    close: () => setTarget(null),
    visible: target !== null,
    items,
    busy,
    message,
  };
}

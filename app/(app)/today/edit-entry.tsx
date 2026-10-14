import { useEffect, useRef, useState } from 'react';
import { Pressable, View } from 'react-native';
import { useLocalSearchParams, useNavigation, useRouter } from 'expo-router';
import { CategoryHeader } from '@/components/entry/CategoryHeader';
import { EntryForm } from '@/components/entry/EntryForm';
import { goToToday } from '@/components/entry/goToToday';
import { useEntry } from '@/components/entry/useEntry';
import { useEntryDraft } from '@/components/entry/useEntryDraft';
import { useUpdateEntry } from '@/components/entry/useUpdateEntry';
import { StatusMessage } from '@/components/today/StatusMessage';
import { AppText } from '@/components/ui/AppText';
import { ConfirmSheet } from '@/components/ui/ConfirmSheet';
import { PrimaryButton } from '@/components/ui/PrimaryButton';
import { Screen } from '@/components/ui/Screen';
import type { Entry } from '@/lib/api/entries';
import { useSession } from '@/lib/session/store';
import { minTouchTarget, space } from '@/lib/theme/tokens';

/**
 * The form of edit entry (03-UI-SPEC section 10), mounted only once the entry has loaded so the
 * draft starts from the entry on its first render. The leave guard is the one `log.tsx` uses:
 * with unsaved changes every way out opens the leave sheet and the iOS swipe is off.
 */
function EditEntryForm({ entry, userId }: { entry: Entry; userId: string | null }) {
  const router = useRouter();
  const navigation = useNavigation();
  // The entry as it was when the form opened: the draft's baseline and what the patch compares to.
  const [original] = useState(entry);
  const draft = useEntryDraft(original);
  const update = useUpdateEntry({ userId, original, draft });

  const [leaveOpen, setLeaveOpen] = useState(false);
  const allowLeaveRef = useRef(false);
  const pendingActionRef = useRef<Parameters<typeof navigation.dispatch>[0] | null>(null);

  useEffect(() => {
    navigation.setOptions({ gestureEnabled: !draft.unsaved });
  }, [navigation, draft.unsaved]);

  const isUnsaved = draft.isUnsaved;
  useEffect(() => {
    return navigation.addListener('beforeRemove', (event) => {
      if (allowLeaveRef.current || !isUnsaved()) return;
      event.preventDefault();
      pendingActionRef.current = event.data.action;
      setLeaveOpen(true);
    });
  }, [navigation, isUnsaved]);

  function cancel() {
    if (update.submitting) return;
    if (router.canGoBack()) router.back();
    else router.replace('/(app)/today');
  }

  function confirmLeave() {
    allowLeaveRef.current = true;
    setLeaveOpen(false);
    const action = pendingActionRef.current;
    pendingActionRef.current = null;
    if (action) navigation.dispatch(action);
    else router.back();
  }

  return (
    <Screen scroll={false}>
      <EntryForm
        mode="edit"
        initial={original}
        draft={draft}
        submitLabel="Save changes"
        submitting={update.submitting}
        message={update.message}
        terminal={update.terminal}
        onTerminal={update.backToToday}
        photoMessage={update.photoMessage}
        onSubmit={() => void update.submit()}
        header={
          <CategoryHeader
            variant="edit"
            localDate={original.local_date}
            categoryName={original.category.name}
            onCancel={cancel}
          />
        }
      />

      <ConfirmSheet
        visible={leaveOpen}
        title="Leave without saving?"
        body="Your changes will be lost."
        confirmLabel="Leave"
        dismissLabel="Keep editing"
        onConfirm={confirmLeave}
        onDismiss={() => {
          pendingActionRef.current = null;
          setLeaveOpen(false);
        }}
      />
    </Screen>
  );
}

/**
 * Edit entry, pushed from 'Edit' on the entry page with the entry's id. It loads the entry, then
 * shows `EditEntryForm`. An id that is not a UUID, or an entry that is gone, shows 'This entry
 * isn't here any more.' with a 'Back to Today' button and never reaches the form. No bottom bar
 * (the Tabs layout hides it for the route name `edit-entry`).
 */
export default function EditEntryRoute() {
  const router = useRouter();
  const params = useLocalSearchParams<{ id?: string | string[] }>();
  const id = (Array.isArray(params.id) ? params.id[0] : params.id) ?? '';
  const { user } = useSession();
  const userId = user?.id ?? null;
  const load = useEntry(id);

  if (load.entry !== null) {
    return <EditEntryForm key={load.entry.id} entry={load.entry} userId={userId} />;
  }

  return (
    <Screen scroll={false}>
      <View style={{ height: minTouchTarget, justifyContent: 'center' }}>
        <Pressable
          onPress={() => (router.canGoBack() ? router.back() : router.replace('/(app)/today'))}
          accessibilityRole="button"
          accessibilityLabel="Cancel"
          hitSlop={{ right: 12 }}
          style={{ height: minTouchTarget, justifyContent: 'center', alignSelf: 'flex-start' }}
        >
          <AppText role="body">Cancel</AppText>
        </Pressable>
      </View>
      <View style={{ marginTop: space.xl }}>
        {load.status === 'loading' ? (
          <AppText role="label" tone="muted">
            Loading your entry.
          </AppText>
        ) : load.status === 'offline' ? (
          <StatusMessage kind={load.failure} onRetry={load.reload} />
        ) : (
          <View>
            <AppText role="label" tone="muted">
              This entry isn't here any more.
            </AppText>
            <View style={{ marginTop: space.lg }}>
              <PrimaryButton label="Back to Today" onPress={() => goToToday(router)} />
            </View>
          </View>
        )}
      </View>
    </Screen>
  );
}

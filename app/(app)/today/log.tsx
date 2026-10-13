import { useEffect, useRef, useState } from 'react';
import { View } from 'react-native';
import { useNavigation, useRouter } from 'expo-router';
import { CategoryHeader } from '@/components/entry/CategoryHeader';
import { EntryForm } from '@/components/entry/EntryForm';
import { useCreateEntry } from '@/components/entry/useCreateEntry';
import { useEntryDraft } from '@/components/entry/useEntryDraft';
import { AppText } from '@/components/ui/AppText';
import { ConfirmSheet } from '@/components/ui/ConfirmSheet';
import { PrimaryButton } from '@/components/ui/PrimaryButton';
import { Screen } from '@/components/ui/Screen';
import { useSession } from '@/lib/session/store';
import { space } from '@/lib/theme/tokens';
import { clearPendingSpin, readToday } from '@/lib/wheel/todayStore';

/** What the screen keeps from Today for the whole visit (plan 03-02 binding decision 5). */
type LoggedSpin = { localDate: string; seq: number; categoryName: string; late: boolean };

/**
 * Today's spin, read once from the store: only a spin whose day is still open to a post (state
 * Result or Late) can be logged. Anything else, or no spin, gives null.
 */
function spinToLog(userId: string | null): LoggedSpin | null {
  const today = readToday(userId).today;
  const spin = today?.spin;
  if (!today || !spin) return null;
  if (today.state !== 'result' && today.state !== 'late') return null;
  return {
    localDate: spin.local_date,
    seq: spin.seq,
    categoryName: spin.category.name,
    late: today.state === 'late',
  };
}

/**
 * The entry screen, pushed from 'Log it' (03-UI-SPEC section 4, the developer's picture 03). A full
 * screen with no bottom bar (the Tabs layout hides it for this route name).
 *
 * It reads Today ONCE, when it opens, and keeps that spin for the whole visit, so the form never
 * changes under the person; if Today moves on meanwhile, the server refuses the save and the
 * message says so. It sends that spin's date and number and never works out the date, the window,
 * the state or whether the post is late.
 *
 * The leave guard is the one Edit wheel uses: a `beforeRemove` listener that, while the draft has
 * unsaved work, stops the exit and opens the leave sheet, with the swipe switched off meanwhile.
 */
export default function LogRoute() {
  const router = useRouter();
  const navigation = useNavigation();
  const { user } = useSession();
  const userId = user?.id ?? null;

  // Read once. A live read would flip to 'nothing to log' the moment the answer of our own post
  // lands in the store, just before the screen goes back.
  const [shown] = useState(() => spinToLog(userId));

  const draft = useEntryDraft();
  const create = useCreateEntry({ userId, spin: shown, draft });

  const [leaveOpen, setLeaveOpen] = useState(false);
  const allowLeaveRef = useRef(false);
  const pendingActionRef = useRef<Parameters<typeof navigation.dispatch>[0] | null>(null);

  // The iOS swipe is off while there is unsaved work; the guard below catches every other way out.
  useEffect(() => {
    navigation.setOptions({ gestureEnabled: !draft.unsaved });
  }, [navigation, draft.unsaved]);

  const isUnsaved = draft.isUnsaved;
  useEffect(() => {
    return navigation.addListener('beforeRemove', (event) => {
      if (allowLeaveRef.current || !isUnsaved()) return;
      event.preventDefault();
      pendingActionRef.current = event.data.action;
      // A pressed Spin button must not start a spin after this sheet (plan 02-02 decision 4).
      clearPendingSpin(userId);
      setLeaveOpen(true);
    });
  }, [navigation, userId, isUnsaved]);

  function goBack() {
    if (router.canGoBack()) router.back();
    else router.replace('/(app)/today');
  }

  function cancel() {
    // Cancel is in the fixed header and stays live while a save runs: leaving then would strand it.
    if (create.submitting) return;
    goBack();
  }

  function confirmLeave() {
    allowLeaveRef.current = true;
    setLeaveOpen(false);
    const action = pendingActionRef.current;
    pendingActionRef.current = null;
    if (action) navigation.dispatch(action);
    else router.back();
  }

  if (shown === null) {
    return (
      <Screen scroll={false}>
        <View style={{ marginTop: space.xl }}>
          <AppText role="label" tone="muted">
            There is nothing to log right now.
          </AppText>
          <View style={{ marginTop: space.lg }}>
            <PrimaryButton label="Back to Today" onPress={goBack} />
          </View>
        </View>
      </Screen>
    );
  }

  return (
    <Screen scroll={false}>
      <EntryForm
        mode="new"
        draft={draft}
        submitLabel="Save"
        submitting={create.submitting}
        message={create.message}
        terminal={create.terminal}
        onTerminal={create.backToToday}
        photoMessage={create.photoMessage}
        onSubmit={() => void create.submit()}
        header={
          <CategoryHeader
            localDate={shown.localDate}
            categoryName={shown.categoryName}
            late={shown.late}
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

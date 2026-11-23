import { useRef, useState } from 'react';
import { ScrollView, TextInput, View } from 'react-native';
import { useRouter } from 'expo-router';
import { AppText } from '@/components/ui/AppText';
import { PrimaryButton } from '@/components/ui/PrimaryButton';
import { Screen } from '@/components/ui/Screen';
import { TextField } from '@/components/ui/TextField';
import { FIELD_VIEW_ID } from '@/components/ui/useKeepFocusedInputInView';
import { BackRow } from '@/components/settings/BackRow';
import { PasswordChecklist } from '@/components/settings/PasswordChecklist';
import { ApiError } from '@/lib/api/client';
import { changePassword } from '@/lib/api/account';
import { announceProblem } from '@/lib/forms/announce';
import { bringIntoView } from '@/lib/forms/bringIntoView';
import { stillNeeded } from '@/lib/forms/rules';
import { setSettingsNotice } from '@/lib/settings/notice';
import {
  CURRENT_PASSWORD_EMPTY_MESSAGE,
  CURRENT_PASSWORD_WRONG_MESSAGE,
  GENERIC_MESSAGE,
  LOOKS_GOOD_MESSAGE,
  NEW_PASSWORD_REFUSED_MESSAGE,
  newPasswordProblem,
  PASSWORD_SHORT_MESSAGE,
  PASSWORD_WEAK_MESSAGE,
  PASSWORD_NOTICE,
} from '@/lib/settings/rules';
import { useSession } from '@/lib/session/store';
import { space } from '@/lib/theme/tokens';

/**
 * The Password page (02.1-UI-SPEC section 6), pushed from the Password row: the current password,
 * one new password (no "type it again", like sign-up), each with its own Show or Hide link.
 *
 * Save is `POST /me/password`. The answer is a log-in shaped body (both tokens and the user) and
 * goes to the session's sign in, so this phone stays signed in on the new tokens while every other
 * phone is signed out. Neither password is put in a route, a log or a notice, and a failed save
 * never clears the boxes.
 */
function isChecklistProblem(problem: string): boolean {
  return problem === PASSWORD_SHORT_MESSAGE || problem === PASSWORD_WEAK_MESSAGE;
}

export function PasswordScreen() {
  const router = useRouter();
  const { signIn } = useSession();
  const [current, setCurrent] = useState('');
  const [next, setNext] = useState('');
  // The new password is judged when its box is left, and after that as it is typed.
  const [nextTouched, setNextTouched] = useState(false);
  // Set by a tap on the grey Save password: from then on the New password box names the checklist's
  // problems too.
  const [asked, setAsked] = useState(false);
  const [currentError, setCurrentError] = useState('');
  const [nextServerError, setNextServerError] = useState('');
  const [formError, setFormError] = useState('');
  const [saving, setSaving] = useState(false);
  const busyRef = useRef(false);
  // A tap on the grey button scrolls to the first box with a problem and puts the cursor in it. The
  // two field views carry FIELD_VIEW_ID, so the keyboard leaves the line and the checklist in view.
  const scrollRef = useRef<ScrollView>(null);
  const currentFieldRef = useRef<View>(null);
  const nextFieldRef = useRef<View>(null);
  const currentBoxRef = useRef<TextInput>(null);
  const nextBoxRef = useRef<TextInput>(null);

  const nextProblem = newPasswordProblem(next, current);
  // The checklist under the box already says what is missing, so only the other problems show under
  // the box, until a tap on the grey button asks for all of them.
  const nextError =
    nextServerError !== ''
      ? nextServerError
      : nextProblem !== '' && (asked || (nextTouched && !isChecklistProblem(nextProblem)))
        ? nextProblem
        : '';
  const nextOk = next !== '' && nextProblem === '' && nextServerError === '';
  const canSave = current !== '' && next !== '' && nextProblem === '' && !saving;
  const currentMissing = current === '';

  function handleCurrentChange(value: string) {
    setCurrent(value);
    setCurrentError('');
    setFormError('');
  }

  function handleNextChange(value: string) {
    setNext(value);
    setNextServerError('');
    setFormError('');
  }

  function handleError(err: unknown) {
    if (!(err instanceof ApiError)) {
      setFormError(GENERIC_MESSAGE);
      return;
    }
    if (err.code === 'validation_failed') {
      // The server names the box: the current password is wrong, or the new one is refused.
      if (err.field === 'current_password') setCurrentError(CURRENT_PASSWORD_WRONG_MESSAGE);
      else setNextServerError(NEW_PASSWORD_REFUSED_MESSAGE);
      return;
    }
    // No connection, too many tries, anything else.
    setFormError(err.userMessage);
  }

  // A tap on the grey Save password: both boxes show their line, touched or not. Then VoiceOver says
  // the first one (current, then new), the page scrolls it into view and the cursor goes into it.
  function showProblems() {
    if (currentMissing) setCurrentError(CURRENT_PASSWORD_EMPTY_MESSAGE);
    setNextTouched(true);
    setAsked(true);
    if (currentMissing) {
      announceProblem(CURRENT_PASSWORD_EMPTY_MESSAGE, 'Current password');
      bringIntoView(scrollRef.current, currentFieldRef.current);
      currentBoxRef.current?.focus();
    } else if (nextProblem !== '') {
      announceProblem(nextProblem, 'New password');
      bringIntoView(scrollRef.current, nextFieldRef.current);
      nextBoxRef.current?.focus();
    }
  }

  async function save() {
    if (!canSave || busyRef.current) return;
    busyRef.current = true;
    setSaving(true);
    setFormError('');
    try {
      const result = await changePassword(current, next);
      await signIn(result);
      setCurrent('');
      setNext('');
      setSettingsNotice(PASSWORD_NOTICE);
      if (router.canGoBack()) router.back();
      else router.replace('/(app)/profile');
    } catch (err) {
      handleError(err);
    } finally {
      busyRef.current = false;
      setSaving(false);
    }
  }

  return (
    <Screen edges={['top', 'left', 'right']} scrollRef={scrollRef}>
      <View style={{ paddingTop: space.lg, paddingBottom: space.xl }}>
        <BackRow />

        <View style={{ marginTop: space.md }}>
          <AppText role="title" accessibilityRole="header">
            Password
          </AppText>
        </View>

        <View ref={currentFieldRef} nativeID={FIELD_VIEW_ID} style={{ marginTop: space.lg }}>
          <TextField
            ref={currentBoxRef}
            label="Current password"
            variant="soft"
            value={current}
            onChangeText={handleCurrentChange}
            error={currentError}
            revealable
            autoCapitalize="none"
            autoCorrect={false}
            autoComplete="current-password"
            textContentType="password"
          />
        </View>

        <View ref={nextFieldRef} nativeID={FIELD_VIEW_ID} style={{ marginTop: space.md }}>
          <TextField
            ref={nextBoxRef}
            label="New password"
            variant="soft"
            value={next}
            onChangeText={handleNextChange}
            onBlur={() => setNextTouched(true)}
            error={nextError}
            success={nextOk ? LOOKS_GOOD_MESSAGE : undefined}
            revealable
            autoCapitalize="none"
            autoCorrect={false}
            autoComplete="new-password"
            textContentType="newPassword"
          />
          <PasswordChecklist value={next} />
        </View>

        {formError !== '' ? (
          <View accessibilityLiveRegion="polite" style={{ marginTop: space.md }}>
            <AppText role="body" tone="default">
              {formError}
            </AppText>
          </View>
        ) : null}

        <View style={{ marginTop: space.lg }}>
          <PrimaryButton
            label="Save password"
            disabled={!canSave}
            loading={saving}
            onPress={save}
            onBlockedPress={showProblems}
            blockedHint={stillNeeded([currentMissing ? 'current password' : '', nextProblem !== '' ? 'new password' : ''])}
          />
        </View>
      </View>
    </Screen>
  );
}

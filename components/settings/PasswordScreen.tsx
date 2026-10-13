import { useRef, useState } from 'react';
import { Pressable, View } from 'react-native';
import { useRouter } from 'expo-router';
import { AppText } from '@/components/ui/AppText';
import { PrimaryButton } from '@/components/ui/PrimaryButton';
import { Screen } from '@/components/ui/Screen';
import { TextField } from '@/components/ui/TextField';
import { BackRow } from '@/components/settings/BackRow';
import { PasswordChecklist } from '@/components/settings/PasswordChecklist';
import { ApiError } from '@/lib/api/client';
import { changePassword } from '@/lib/api/account';
import { setSettingsNotice } from '@/lib/settings/notice';
import {
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
  const [showCurrent, setShowCurrent] = useState(false);
  const [showNext, setShowNext] = useState(false);
  // The new password is judged when its box is left, and after that as it is typed.
  const [nextTouched, setNextTouched] = useState(false);
  const [currentError, setCurrentError] = useState('');
  const [nextServerError, setNextServerError] = useState('');
  const [formError, setFormError] = useState('');
  const [saving, setSaving] = useState(false);
  const busyRef = useRef(false);

  const nextProblem = newPasswordProblem(next, current);
  // The checklist under the box already says what is missing, so only the other problems show red.
  const nextError =
    nextServerError !== '' ? nextServerError : nextTouched && nextProblem !== '' && !isChecklistProblem(nextProblem) ? nextProblem : '';
  const nextOk = next !== '' && nextProblem === '' && nextServerError === '';
  const canSave = current !== '' && next !== '' && nextProblem === '' && !saving;

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
    <Screen edges={['top', 'left', 'right']}>
      <View style={{ paddingTop: space.lg, paddingBottom: space.xl }}>
        <BackRow />

        <View style={{ marginTop: space.md }}>
          <AppText role="title" accessibilityRole="header">
            Password
          </AppText>
        </View>

        <View style={{ marginTop: space.lg }}>
          <TextField
            label="Current password"
            variant="soft"
            value={current}
            onChangeText={handleCurrentChange}
            error={currentError}
            secureTextEntry={!showCurrent}
            autoCapitalize="none"
            autoCorrect={false}
            autoComplete="current-password"
            textContentType="password"
          />
          {/* Sits on the label row, where the other fields show their hint. */}
          <Pressable
            onPress={() => setShowCurrent((value) => !value)}
            accessibilityRole="button"
            accessibilityLabel={showCurrent ? 'Hide current password' : 'Show current password'}
            hitSlop={12}
            style={{ position: 'absolute', top: 0, right: 0 }}
          >
            <AppText role="label" tone="muted">
              {showCurrent ? 'Hide' : 'Show'}
            </AppText>
          </Pressable>
        </View>

        <View style={{ marginTop: space.md }}>
          <TextField
            label="New password"
            variant="soft"
            value={next}
            onChangeText={handleNextChange}
            onBlur={() => setNextTouched(true)}
            error={nextError}
            success={nextOk ? LOOKS_GOOD_MESSAGE : undefined}
            secureTextEntry={!showNext}
            autoCapitalize="none"
            autoCorrect={false}
            autoComplete="new-password"
            textContentType="newPassword"
          />
          <Pressable
            onPress={() => setShowNext((value) => !value)}
            accessibilityRole="button"
            accessibilityLabel={showNext ? 'Hide new password' : 'Show new password'}
            hitSlop={12}
            style={{ position: 'absolute', top: 0, right: 0 }}
          >
            <AppText role="label" tone="muted">
              {showNext ? 'Hide' : 'Show'}
            </AppText>
          </Pressable>
          <PasswordChecklist value={next} />
        </View>

        {formError !== '' ? (
          <View accessibilityLiveRegion="polite" style={{ marginTop: space.md }}>
            <AppText role="body" tone="destructive">
              {formError}
            </AppText>
          </View>
        ) : null}

        <View style={{ marginTop: space.lg }}>
          <PrimaryButton label="Save password" disabled={!canSave} loading={saving} onPress={save} />
        </View>
      </View>
    </Screen>
  );
}

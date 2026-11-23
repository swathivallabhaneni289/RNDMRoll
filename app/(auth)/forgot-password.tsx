import { useEffect, useRef, useState } from 'react';
import { Pressable, View } from 'react-native';
import { useLocalSearchParams, useNavigation, useRouter } from 'expo-router';
import { Ionicons } from '@expo/vector-icons';
import { PasswordChecklist } from '@/components/settings/PasswordChecklist';
import { AppText } from '@/components/ui/AppText';
import { PrimaryButton } from '@/components/ui/PrimaryButton';
import { Screen } from '@/components/ui/Screen';
import { TextField } from '@/components/ui/TextField';
import { api, ApiError } from '@/lib/api/client';
import type { ResetResult } from '@/lib/api/types';
import { answerLost, codeLine, maybeSpentByLostReset, SHORT_CODE, tooMany } from '@/lib/auth/resetPage';
import { RESTORED_MESSAGE } from '@/lib/auth/restored';
import { passwordProblem } from '@/lib/settings/rules';
import { color, space } from '@/lib/theme/tokens';

const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

/** The email Log in passed along, when it is one; '' for none, a username or anything else. */
function carriedEmail(param: string | string[] | undefined): string {
  const typed = typeof param === 'string' ? param.trim() : '';
  return EMAIL_PATTERN.test(typed) ? typed : '';
}

const GENERIC = 'Something went wrong. Please try again.';
/** A 429 on this page: the server's limits count requests here, not wrong codes. */
const TOO_MANY = 'Too many requests. Wait a minute and try again.';
// The lines for the server's answers about the code, and the rules that pick them, are in
// lib/auth/resetPage.ts (codeLine, maybeSpentByLostReset, tooMany, answerLost).
/** The email step's line under its heading. It reads right both for an empty box and for one filled
 * in from Log in, so it never asks for an email that is already there (developer, 2026-11-23). */
const EMAIL_HINT = "We'll email you a code to reset your password.";
/** The code step's line under its heading when no notice is set (a sent code's notice takes its place). */
const CODE_HINT = 'Enter the 6-digit code from the email.';
/** The password step's lines under its heading: green, then muted. */
const CODE_VERIFIED = 'Code verified.';
const CHOOSE_PASSWORD = 'Choose a new password.';
/** No answer, or the server failed: the password may have changed all the same. */
const MAYBE_CHANGED = 'Something went wrong. If your password did change, log in with the new one.';
/**
 * A code answered as wrong or used, by the check or by a reset, after a reset whose answer was
 * lost: that reset may have changed the password and spent the code, so the plain line about the
 * code could be wrong.
 */
const MAYBE_CHANGED_AFTER = 'If your password did change, log in with the new one.';
/** The done step's line: the password is set, and nobody is signed in by it. */
const LOG_IN_WITH_NEW = 'Log in with your new password.';

/**
 * Forgot password (developer, 2026-10-21), opened from Log in. Three steps on one page, as the new
 * password comes only after the code is checked (developer, 2026-11-13): your email, then the
 * 6-digit code from the email, which the server checks without using it up, then a new password
 * (the same rules as sign-up, with the checklist). Set new password sends the checked code again,
 * and the server checks it once more before it spends it: if it expired, was replaced or was used
 * in between, the page goes back to the code step, which has Send a new code, and keeps the typed
 * password (after a lost answer, maybeChangedRef says which line it shows). Setting the new
 * password ends every session the account had and signs nobody in, and the server emails the owner
 * that it changed: the page says "Password changed" and its Log in goes back to Log in with this
 * email filled in, for the new password (developer, 2026-11-23, after OWASP's "Don't automatically
 * log the user in"). The server answers the same for any email, so the page never says whether an
 * account exists. A code that set the password is never sent again. A reset within 30 days of
 * Delete account keeps the account, and the done step says so.
 */
export default function ForgotPasswordScreen() {
  const router = useRouter();
  const navigation = useNavigation();
  const params = useLocalSearchParams<{ email?: string | string[] }>();
  const [step, setStep] = useState<'email' | 'code' | 'password' | 'done'>('email');
  // An email typed on Log in comes filled in, so the page asks only for Send code (developer,
  // 2026-11-20). Read once, like autoFocus: the keyboard opens by itself only for an empty box.
  const [carried] = useState(() => carriedEmail(params.email));
  const [email, setEmail] = useState(carried);
  const [code, setCode] = useState('');
  const [password, setPassword] = useState('');
  const [emailError, setEmailError] = useState<string | undefined>();
  const [codeError, setCodeError] = useState<string | undefined>();
  const [passwordError, setPasswordError] = useState<string | undefined>();
  const [formError, setFormError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  // The reset also cancelled a pending deletion: the done step says the account stays.
  const [restored, setRestored] = useState(false);
  // Set as soon as a reset's answer says the code set the password, before anything else is
  // awaited: from then on the spent code is never sent again, not even to be checked, whatever
  // happens next.
  const spentRef = useRef(false);
  // True from a tap until its request is answered, set before anything is awaited: a second tap
  // (on any button) before the page has drawn busy sends nothing.
  const sendingRef = useRef(false);
  // A reset whose answer was lost (no connection, or the server failed): it may have changed the
  // password. Until a new code is sent, a later answer, of the check or of a reset, that the code
  // is wrong or used says so again instead of the plain line about the code.
  const maybeChangedRef = useRef(false);
  // True once logIn has started leaving, so the guard below lets that one through.
  const leavingRef = useRef(false);

  // Once the password is set, every way back goes where Log in does: the iOS swipe is off on the
  // done step, and the guard turns any other way out (Android's back) into logIn.
  useEffect(() => {
    navigation.setOptions({ gestureEnabled: step !== 'done' });
  }, [navigation, step]);
  useEffect(() => {
    if (step !== 'done') return;
    return navigation.addListener('beforeRemove', (event) => {
      if (leavingRef.current) return;
      event.preventDefault();
      logIn();
    });
    // logIn reads only the email, which the done step never changes.
  }, [navigation, step]);

  function goBack() {
    // Once the password is set, Back goes where Log in does.
    if (step === 'done') logIn();
    else if (router.canGoBack()) router.back();
    else router.replace('/login');
  }

  // Back to the Log in page this one was opened from (or a new one), with this email filled in and
  // an empty password box. `reset` is new each time, so a second reset with the same email fills
  // it in again (login.tsx).
  function logIn() {
    leavingRef.current = true;
    router.dismissTo({ pathname: '/login', params: { email: email.trim(), reset: String(Date.now()) } });
  }

  function failure(err: unknown): string {
    if (tooMany(err)) return TOO_MANY;
    return err instanceof ApiError && err.code === 'network_unavailable' ? err.userMessage : GENERIC;
  }

  async function sendCode() {
    if (sendingRef.current) return;
    const typed = email.trim();
    if (!EMAIL_PATTERN.test(typed)) {
      setEmailError('Enter a valid email address.');
      return;
    }
    sendingRef.current = true;
    setEmailError(undefined);
    setFormError(null);
    setBusy(true);
    try {
      await api.post('/auth/password/forgot', { email: typed }, { auth: false });
      // The answer is the same whether or not a code is emailed (no account, a code under a minute
      // old, or one whose 5 tries are used up), and so is this line, for any address. That keeps it
      // from saying whether the email has an account without an "If an account exists" (developer,
      // 2026-11-20: the usual wording), as Login.gov, Okta, Keycloak, Supabase and Ory do. It names
      // the address, so a typo shows. An older code typed after a new one replaced it gets its own
      // line (REPLACED).
      setNotice(`We sent a code to ${typed}.`);
      // A message about a code typed before would contradict the line above.
      setCodeError(undefined);
      maybeChangedRef.current = false;
      setStep('code');
    } catch (err) {
      setFormError(failure(err));
    } finally {
      sendingRef.current = false;
      setBusy(false);
    }
  }

  async function verifyCode() {
    if (spentRef.current || sendingRef.current) return;
    const digits = code.replace(/\D/g, '');
    const nextCodeError = digits.length === 6 ? undefined : SHORT_CODE;
    setCodeError(nextCodeError);
    if (nextCodeError) return;
    sendingRef.current = true;
    setFormError(null);
    setBusy(true);
    try {
      // The server checks the code the way a reset does (a wrong one costs a try) but never spends
      // it, so the same code then sets the password.
      await api.post('/auth/password/verify', { email: email.trim(), code: digits }, { auth: false });
      // The digits it checked, not what the box holds now: the box can change while the check runs,
      // and the password step sends this code again.
      setCode(digits);
      setStep('password');
    } catch (err) {
      if (err instanceof ApiError && err.field === 'code') {
        if (maybeSpentByLostReset(err.reason, maybeChangedRef.current)) {
          // Wrong after a lost answer: that reset may have spent the code and changed the password.
          setFormError(MAYBE_CHANGED_AFTER);
        } else {
          setCodeError(codeLine(err.reason));
        }
      } else {
        // The check changes nothing, so no other answer can mean the password changed.
        setFormError(failure(err));
      }
    } finally {
      sendingRef.current = false;
      setBusy(false);
    }
  }

  async function reset() {
    if (spentRef.current || sendingRef.current) return;
    // The code was checked on the code step and cannot change on this one, so only the password is
    // checked here. The server checks the code again before it spends it.
    const nextPasswordError = passwordProblem(password) || undefined;
    setPasswordError(nextPasswordError);
    if (nextPasswordError) return;
    sendingRef.current = true;
    setFormError(null);
    setBusy(true);
    let result: ResetResult;
    try {
      result = await api.post<ResetResult>(
        '/auth/password/reset',
        { email: email.trim(), code, new_password: password },
        { auth: false }
      );
    } catch (err) {
      if (err instanceof ApiError && err.field === 'code') {
        if (maybeSpentByLostReset(err.reason, maybeChangedRef.current)) {
          // Wrong or used after a lost answer: that reset may have changed the password. This line
          // takes the place of the code's, so the Code box gets none.
          setFormError(MAYBE_CHANGED_AFTER);
        } else {
          setCodeError(codeLine(err.reason));
        }
        // Either way the checked code expired, was replaced or was used since: back to the code
        // step, which has Send a new code and no "Code verified." line. The typed password stays,
        // so a code checked again needs no retyping.
        setStep('code');
      } else if (err instanceof ApiError && err.field === 'new_password') {
        setPasswordError(err.userMessage);
      } else if (tooMany(err)) {
        setFormError(TOO_MANY);
      } else {
        // No answer, or the server failed: the change may have gone through all the same.
        if (answerLost(err)) maybeChangedRef.current = true;
        setFormError(MAYBE_CHANGED);
      }
      sendingRef.current = false;
      setBusy(false);
      return;
    }
    spentRef.current = true;
    setRestored(result.restored === true);
    setStep('done');
    sendingRef.current = false;
    setBusy(false);
  }

  return (
    <Screen>
      <View style={{ paddingTop: space.lg }}>
        <Pressable
          onPress={goBack}
          accessibilityRole="button"
          accessibilityLabel="Back"
          hitSlop={12}
          style={{ flexDirection: 'row', alignItems: 'center', alignSelf: 'flex-start' }}
        >
          <Ionicons name="chevron-back" size={22} color={color.ink} />
          <AppText role="body">Back</AppText>
        </Pressable>

        <View style={{ marginTop: space.xl }}>
          <AppText role="heading" accessibilityRole="header">
            {step === 'email'
              ? 'Forgot password'
              : step === 'code'
                ? 'Check your email'
                : step === 'password'
                  ? 'Set a new password'
                  : 'Password changed'}
          </AppText>
        </View>
        {step === 'email' || step === 'code' ? (
          <View style={{ marginTop: space.sm }}>
            <AppText role="body" tone="muted">
              {step === 'email' ? EMAIL_HINT : notice ?? CODE_HINT}
            </AppText>
          </View>
        ) : null}
        {step === 'password' ? (
          <>
            <View style={{ marginTop: space.sm }}>
              <AppText role="body" tone="success">
                {CODE_VERIFIED}
              </AppText>
            </View>
            <View style={{ marginTop: space.xs }}>
              <AppText role="body" tone="muted">
                {CHOOSE_PASSWORD}
              </AppText>
            </View>
          </>
        ) : null}
        {step === 'done' ? (
          <>
            {restored ? (
              <View style={{ marginTop: space.sm }}>
                <AppText role="body">{RESTORED_MESSAGE}</AppText>
              </View>
            ) : null}
            <View style={{ marginTop: restored ? space.xs : space.sm }}>
              <AppText role="body" tone="muted">
                {LOG_IN_WITH_NEW}
              </AppText>
            </View>
          </>
        ) : null}

        {step === 'email' ? (
          <View style={{ marginTop: space.lg }}>
            <TextField
              label="Email"
              value={email}
              onChangeText={setEmail}
              error={emailError}
              keyboardType="email-address"
              autoCapitalize="none"
              autoComplete="email"
              textContentType="emailAddress"
              autoFocus={carried === ''}
            />
          </View>
        ) : null}
        {/* Each box keeps its own slot and key: it mounts on its own step, so autoFocus (read only
            on mount) moves the keyboard to it, also on the way back to the code. */}
        {step === 'code' ? (
          <View key="code" style={{ marginTop: space.lg }}>
            <TextField
              label="Code"
              value={code}
              onChangeText={(next) => setCode(next.replace(/\D/g, '').slice(0, 6))}
              error={codeError}
              keyboardType="number-pad"
              autoComplete="one-time-code"
              textContentType="oneTimeCode"
              maxLength={6}
              autoFocus
            />
          </View>
        ) : null}
        {step === 'password' ? (
          <View key="new-password" style={{ marginTop: space.lg }}>
            <TextField
              label="New password"
              value={password}
              onChangeText={setPassword}
              error={passwordError}
              revealable
              maxLength={72}
              autoComplete="new-password"
              textContentType="newPassword"
              autoFocus
            />
            <PasswordChecklist value={password} />
          </View>
        ) : null}

        {formError ? (
          <View style={{ marginTop: space.md }} accessibilityLiveRegion="polite">
            <AppText role="body">{formError}</AppText>
          </View>
        ) : null}

        <View style={{ marginTop: space.lg }}>
          {step === 'email' ? (
            <PrimaryButton label="Send code" onPress={sendCode} loading={busy} disabled={busy} />
          ) : step === 'code' ? (
            <PrimaryButton label="Verify code" onPress={verifyCode} loading={busy} disabled={busy} />
          ) : step === 'password' ? (
            <PrimaryButton label="Set new password" onPress={reset} loading={busy} disabled={busy} />
          ) : (
            <PrimaryButton label="Log in" onPress={logIn} />
          )}
        </View>

        {step === 'code' ? (
          <Pressable
            onPress={() => void sendCode()}
            disabled={busy}
            accessibilityRole="button"
            hitSlop={8}
            style={{ marginTop: space.md, alignSelf: 'center', minHeight: 44, justifyContent: 'center' }}
          >
            <AppText role="button">Send a new code</AppText>
          </Pressable>
        ) : null}
      </View>
    </Screen>
  );
}

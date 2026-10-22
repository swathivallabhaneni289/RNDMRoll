import { useRef, useState } from 'react';
import { Pressable, View } from 'react-native';
import { useRouter } from 'expo-router';
import { Ionicons } from '@expo/vector-icons';
import { PasswordChecklist } from '@/components/settings/PasswordChecklist';
import { AppText } from '@/components/ui/AppText';
import { PrimaryButton } from '@/components/ui/PrimaryButton';
import { Screen } from '@/components/ui/Screen';
import { TextField } from '@/components/ui/TextField';
import { api, ApiError } from '@/lib/api/client';
import type { AuthResult } from '@/lib/api/types';
import { useSession } from '@/lib/session/store';
import { passwordProblem } from '@/lib/settings/rules';
import { color, space } from '@/lib/theme/tokens';

const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
const GENERIC = 'Something went wrong. Please try again.';
/** A 429 on this page: the server's limits count requests here, not wrong codes. */
const TOO_MANY = 'Too many requests. Wait a minute and try again.';
/**
 * The lines under the Code box, one for each answer about the code (the server's reason). The phone
 * does not count tries (it cannot see the server's count), so each line is true whatever the server
 * holds, and none says whether the email has an account. The server keeps wrong tries across new
 * codes until the code expires (30 minutes after the last code went out), so the line says so.
 */
const WRONG_CODE =
  "That code isn't right or has expired. Wrong tries add up, even across new codes. After 5, you can send a new code 30 minutes after the last one was sent.";
const REPLACED = 'That code was replaced by a newer one. Use the newest code in your email.';
const USED = 'That code was already used. If your password changed, log in with the new one.';
const SHORT_CODE = 'Enter the 6-digit code from the email.';
/** No answer, or the server failed: the password may have changed all the same. */
const MAYBE_CHANGED = 'Something went wrong. If your password did change, log in with the new one.';
/**
 * A code answered as wrong or used after a reset whose answer was lost: that reset may have changed
 * the password and spent the code, so the plain line about the code could be wrong.
 */
const MAYBE_CHANGED_AFTER = 'If your password did change, log in with the new one.';
/** The new password is set, but this phone could not keep the new session. */
const SET_LOG_IN = 'Your new password is set. Log in with it.';

/** The line for the server's reason about the code; an unknown one reads as a wrong code. */
function codeLine(reason: string | undefined): string {
  if (reason === 'replaced') return REPLACED;
  if (reason === 'used') return USED;
  if (reason === 'invalid') return SHORT_CODE;
  return WRONG_CODE;
}

/** Too many requests: by the HTTP status too, as a 429 without the usual body has no code. */
function tooMany(err: unknown): boolean {
  return err instanceof ApiError && (err.status === 429 || err.code === 'rate_limited');
}

/** No answer came (no connection, the client's own time limit) or the server failed. */
function answerLost(err: unknown): boolean {
  return !(err instanceof ApiError) || err.status === 0 || err.status >= 500;
}

/**
 * Forgot password (developer, 2026-10-21), opened from Log in. Two steps on one page: your email,
 * then the 6-digit code from the email and a new password (the same rules as sign-up, with the
 * checklist). The new password signs you in at once and ends every other session. The server
 * answers the same for any email, so the page never says whether an account exists. A code that
 * worked is never sent again: if this phone then cannot keep the new session, the page says the
 * password is set and offers Log in instead.
 */
export default function ForgotPasswordScreen() {
  const router = useRouter();
  const { signIn } = useSession();
  const [step, setStep] = useState<'email' | 'code' | 'done'>('email');
  const [email, setEmail] = useState('');
  const [code, setCode] = useState('');
  const [password, setPassword] = useState('');
  const [emailError, setEmailError] = useState<string | undefined>();
  const [codeError, setCodeError] = useState<string | undefined>();
  const [passwordError, setPasswordError] = useState<string | undefined>();
  const [formError, setFormError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  // Set as soon as the server takes the code, before anything else is awaited: from then on the
  // spent code is never sent again, whatever happens next.
  const spentRef = useRef(false);
  // True from a tap until its request is answered, set before anything is awaited: a second tap
  // (on either button) before the page has drawn busy sends nothing.
  const sendingRef = useRef(false);
  // A reset whose answer was lost (no connection, or the server failed): it may have changed the
  // password. Until a new code is sent, a later answer that the code is wrong or used says so
  // again instead of the plain line about the code.
  const maybeChangedRef = useRef(false);

  function goBack() {
    if (router.canGoBack()) router.back();
    else router.replace('/login');
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
      // old, or one whose 5 tries are used up), so the line says only what is always true. A new
      // code replaces the last one, so the newest code in the email is the one that works.
      setNotice(`If ${typed} has an active account, check your email for a 6-digit code. Use the newest one.`);
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

  async function reset() {
    if (spentRef.current || sendingRef.current) return;
    const digits = code.replace(/\D/g, '');
    const nextCodeError = digits.length === 6 ? undefined : SHORT_CODE;
    const nextPasswordError = passwordProblem(password) || undefined;
    setCodeError(nextCodeError);
    setPasswordError(nextPasswordError);
    if (nextCodeError || nextPasswordError) return;
    sendingRef.current = true;
    setFormError(null);
    setBusy(true);
    let result: AuthResult;
    try {
      result = await api.post<AuthResult>(
        '/auth/password/reset',
        { email: email.trim(), code: digits, new_password: password },
        { auth: false }
      );
    } catch (err) {
      const reason = err instanceof ApiError && err.field === 'code' ? err.reason : undefined;
      if (
        err instanceof ApiError &&
        err.field === 'code' &&
        maybeChangedRef.current &&
        reason !== 'replaced' &&
        reason !== 'invalid'
      ) {
        // Wrong or used after a lost answer: that reset may have changed the password.
        setFormError(MAYBE_CHANGED_AFTER);
      } else if (err instanceof ApiError && err.field === 'code') {
        setCodeError(codeLine(reason));
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
    try {
      // The root guard routes the signed-in account to the app.
      await signIn(result);
    } catch {
      // The password did change; only keeping the new session on this phone failed.
      setStep('done');
      setFormError(SET_LOG_IN);
    } finally {
      sendingRef.current = false;
      setBusy(false);
    }
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
            {step === 'email' ? 'Forgot password' : 'Set a new password'}
          </AppText>
        </View>
        {step === 'done' ? null : (
          <View style={{ marginTop: space.sm }}>
            <AppText role="body" tone="muted">
              {step === 'email'
                ? "Enter your account's email to ask for a code to set a new password."
                : notice ?? 'Enter the code from the email and a new password.'}
            </AppText>
          </View>
        )}

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
              autoFocus
            />
          </View>
        ) : null}
        {step === 'code' ? (
          <>
            <View style={{ marginTop: space.lg }}>
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
            <View style={{ marginTop: space.md }}>
              <TextField
                label="New password"
                value={password}
                onChangeText={setPassword}
                error={passwordError}
                secureTextEntry
                maxLength={72}
                autoComplete="new-password"
                textContentType="newPassword"
              />
              <PasswordChecklist value={password} />
            </View>
          </>
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
            <PrimaryButton label="Set new password" onPress={reset} loading={busy} disabled={busy} />
          ) : (
            <PrimaryButton label="Log in" onPress={goBack} />
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

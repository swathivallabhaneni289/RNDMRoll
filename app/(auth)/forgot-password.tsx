import { useState } from 'react';
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
import { codeUsedUp, noteCodeAsked, noteCodeSpent, noteWrongCode } from '@/lib/auth/resetTries';
import { useSession } from '@/lib/session/store';
import { passwordProblem } from '@/lib/settings/rules';
import { color, space } from '@/lib/theme/tokens';

const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
const GENERIC = 'Something went wrong. Please try again.';
/** The one line under the Code box from the fifth wrong code until a new code can be sent. */
const USED_UP = "After 5 wrong tries the code can't be used any more. You can ask for a new one about 30 minutes after the last one was sent.";

/**
 * Forgot password (developer, 2026-10-21), opened from Log in. Two steps on one page: your email,
 * then the 6-digit code from the email and a new password (the same rules as sign-up, with the
 * checklist). The new password signs you in at once and ends every other session. The server
 * answers the same for any email, so the page never says whether an account exists.
 */
export default function ForgotPasswordScreen() {
  const router = useRouter();
  const { signIn } = useSession();
  const [step, setStep] = useState<'email' | 'code'>('email');
  const [email, setEmail] = useState('');
  const [code, setCode] = useState('');
  const [password, setPassword] = useState('');
  const [emailError, setEmailError] = useState<string | undefined>();
  const [codeError, setCodeError] = useState<string | undefined>();
  const [passwordError, setPasswordError] = useState<string | undefined>();
  const [formError, setFormError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  // 5 wrong codes for this email, until 30 minutes after the code was last asked for
  // (lib/auth/resetTries.ts): the Code box says so in one line, and nothing above it points at the code.
  const [usedUp, setUsedUp] = useState(false);
  const [busy, setBusy] = useState(false);

  function goBack() {
    if (router.canGoBack()) router.back();
    else router.replace('/login');
  }

  function failure(err: unknown): string {
    return err instanceof ApiError && (err.code === 'network_unavailable' || err.code === 'rate_limited') ? err.userMessage : GENERIC;
  }

  async function sendCode() {
    const typed = email.trim();
    if (!EMAIL_PATTERN.test(typed)) {
      setEmailError('Enter a valid email address.');
      return;
    }
    setEmailError(undefined);
    setFormError(null);
    // The code is used up: the server sends no new one until it expires, so nothing is asked, and
    // the page says so again, also on a first send after leaving and coming back.
    if (codeUsedUp(typed)) {
      setUsedUp(true);
      setCodeError(undefined);
      setStep('code');
      return;
    }
    setBusy(true);
    try {
      await api.post('/auth/password/forgot', { email: typed }, { auth: false });
      noteCodeAsked(typed);
      // The answer is the same whether or not a code is emailed, so nothing here says one is on its
      // way. A new code goes out only when the last code went out over a minute ago and is not used
      // up (5 tries); a used-up code gets no new one until it expires, 30 minutes after it went out.
      // A new code replaces the last one, so the newest code in the email is the one that works.
      setNotice(
        step === 'code'
          ? `If ${typed} has an account, use the newest code in your email. No new code comes within a minute of the last code, or for up to 30 minutes after 5 wrong tries.`
          : `If ${typed} has an account, check your email for a 6-digit code.`
      );
      // A message about a code typed before would contradict the notice above.
      setCodeError(undefined);
      setUsedUp(false);
      setStep('code');
    } catch (err) {
      setFormError(failure(err));
    } finally {
      setBusy(false);
    }
  }

  async function reset() {
    const digits = code.replace(/\D/g, '');
    const nextCodeError = digits.length === 6 ? undefined : 'Enter the 6-digit code from the email.';
    const nextPasswordError = passwordProblem(password) || undefined;
    setCodeError(nextCodeError);
    setPasswordError(nextPasswordError);
    if (nextCodeError || nextPasswordError) return;
    setFormError(null);
    setBusy(true);
    try {
      const result = await api.post<AuthResult>(
        '/auth/password/reset',
        { email: email.trim(), code: digits, new_password: password },
        { auth: false }
      );
      // The code is spent: the next one for this email starts from 0 tries.
      noteCodeSpent(email);
      // The root guard routes the signed-in account to the app.
      await signIn(result);
    } catch (err) {
      // A wrong, replaced, expired or used-up code, and an email with no account, get the same
      // answer. Every code typed counts as a try; after 5 the code stops working, and no new one is
      // sent until it expires, 30 minutes after it went out. Counted on this phone too, so from the
      // fifth the Code box says the code is used up, also after leaving and coming back.
      if (err instanceof ApiError && err.field === 'code') {
        const nowUsedUp = noteWrongCode(email);
        setUsedUp(nowUsedUp);
        setCodeError(
          nowUsedUp
            ? undefined
            : "That code isn't right or has expired. After 5 wrong tries, wait up to 30 minutes, then send a new code."
        );
      }
      else if (err instanceof ApiError && err.field === 'new_password') setPasswordError(err.userMessage);
      else setFormError(failure(err));
    } finally {
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
        {/* None while the code is used up: the line under the Code box is the only one about it. */}
        {step === 'code' && usedUp ? null : (
          <View style={{ marginTop: space.sm }}>
            <AppText role="body" tone="muted">
              {step === 'email'
                ? "Enter your account's email. We'll send you a code to set a new password."
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
        ) : (
          <>
            <View style={{ marginTop: space.lg }}>
              <TextField
                label="Code"
                value={code}
                onChangeText={(next) => setCode(next.replace(/\D/g, '').slice(0, 6))}
                error={codeError ?? (usedUp ? USED_UP : undefined)}
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
        )}

        {formError ? (
          <View style={{ marginTop: space.md }} accessibilityLiveRegion="polite">
            <AppText role="body">{formError}</AppText>
          </View>
        ) : null}

        <View style={{ marginTop: space.lg }}>
          {step === 'email' ? (
            <PrimaryButton label="Send code" onPress={sendCode} loading={busy} disabled={busy} />
          ) : (
            <PrimaryButton label="Set new password" onPress={reset} loading={busy} disabled={busy} />
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

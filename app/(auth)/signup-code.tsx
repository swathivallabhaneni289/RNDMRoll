import { useRef, useState } from 'react';
import { Pressable, View } from 'react-native';
import { Redirect, useRouter } from 'expo-router';
import { Ionicons } from '@expo/vector-icons';
import { AppText } from '@/components/ui/AppText';
import { PrimaryButton } from '@/components/ui/PrimaryButton';
import { Screen } from '@/components/ui/Screen';
import { TextField } from '@/components/ui/TextField';
import { api, ApiError, isRetryable } from '@/lib/api/client';
import type { AuthResult } from '@/lib/api/types';
import {
  clearPendingSignup,
  lostSignup,
  pendingSignup,
  setLostSignup,
  type PendingSignup,
} from '@/lib/auth/pendingSignup';
import { sayIfRestored } from '@/lib/auth/restored';
import { answerLine, codeLine, GENERIC, sentLine, SHORT_CODE } from '@/lib/auth/signupCode';
import { opensExtras, shouldTryLogin, signupAnswerLost, type SignupTry } from '@/lib/auth/signupRetry';
import { useSession } from '@/lib/session/store';
import { color, space } from '@/lib/theme/tokens';

/**
 * Sign-up's second page, for the 6-digit code emailed at Continue on page 1 (developer, 2026-11-24:
 * sign-up should "work like any other website", and Instagram emails a code during sign-up).
 * Verify code sends it, and only then does the server make the account, which signs in here as
 * Continue once did: the photo-and-bio page follows. Send a new code asks for another, which
 * replaces the last. Back goes to page 1, which still has everything typed; its Continue starts a
 * new sign-up with a new code. The sign-up comes from page 1 in memory only
 * (lib/auth/pendingSignup.ts), so after an app restart this page hands over to page 1. It looks
 * like the code step of Forgot password.
 */
export default function SignupCodeScreen() {
  // Read once, on mount: signing in clears it, and the page draws again on its way out.
  const [signup] = useState(pendingSignup);
  if (signup === null) return <Redirect href="/make-it-yours" />;
  return <CodeStep signup={signup} />;
}

function CodeStep({ signup }: { signup: PendingSignup }) {
  const router = useRouter();
  const { signIn } = useSession();
  const [code, setCode] = useState('');
  const [codeError, setCodeError] = useState<string | undefined>();
  const [formError, setFormError] = useState<string | null>(null);
  // Send a new code was answered: the line under the heading says a new code went out.
  const [resent, setResent] = useState(false);
  const [busy, setBusy] = useState(false);
  // True from a tap until its request is answered, set before anything is awaited: a second tap
  // (on either button) before the page has drawn busy sends nothing.
  const sendingRef = useRef(false);

  function goBack() {
    if (router.canGoBack()) router.back();
    else router.replace('/make-it-yours');
  }

  function failure(err: unknown): string {
    if (!(err instanceof ApiError)) return GENERIC;
    return err.code === 'network_unavailable' ? err.userMessage : answerLine(err);
  }

  async function verifyCode() {
    if (sendingRef.current) return;
    const digits = code.replace(/\D/g, '');
    const nextCodeError = digits.length === 6 ? undefined : SHORT_CODE;
    setCodeError(nextCodeError);
    if (nextCodeError) return;
    sendingRef.current = true;
    setFormError(null);
    setBusy(true);
    try {
      let result: AuthResult;
      // The landing route opens on the photo-and-bio page for this one sign-in.
      let extras = true;
      try {
        result = await api.post<AuthResult>(
          '/auth/signup/verify',
          { signup_token: signup.token, code: digits },
          { auth: false }
        );
        // The account exists now: should this phone fail to keep its session, the next Verify code
        // is answered "expired" (and Continue on page 1, email_taken) and logs in to it.
        setLostSignup(signup);
      } catch (err) {
        // No answer, a server failure, or an answer that could not be read: the server may have
        // made the account all the same.
        if (!(err instanceof ApiError) || signupAnswerLost(err)) {
          setLostSignup(signup);
          throw err;
        }
        const lost = lostSignup();
        if (lost === null || !shouldTryLogin(lost, signup.email, signup.password, err)) throw err;
        // Most likely the person's own new account, made by that verify.
        result = await logInToLostSignup(lost, err);
        extras = opensExtras(lost, result);
      }

      await signIn(result, { extras });
      clearPendingSignup();
      // Only a log in can bring an account back from deletion: said once, as on the Log in page.
      sayIfRestored(result);
    } catch (err) {
      if (err instanceof ApiError && err.field === 'code') setCodeError(codeLine(err.reason));
      else setFormError(failure(err));
    } finally {
      sendingRef.current = false;
      setBusy(false);
    }
  }

  // Logs in with the lost sign-up's email and password, as Continue on page 1 does. A refusal (the
  // email is someone else's, or the password is not that account's) gives back the verify's answer,
  // which shows its own line. A failure that could pass (no answer, too many tries, a server
  // failure) shows its own message, and the next Verify code tries again.
  async function logInToLostSignup(attempt: SignupTry, taken: ApiError): Promise<AuthResult> {
    try {
      return await api.post<AuthResult>(
        '/auth/login',
        { login: attempt.email, password: attempt.password },
        { auth: false }
      );
    } catch (err) {
      throw isRetryable(err) ? err : taken;
    }
  }

  async function sendNewCode() {
    if (sendingRef.current) return;
    sendingRef.current = true;
    setFormError(null);
    setBusy(true);
    try {
      // The server answers the same when the last code is under a minute old and still stands, so
      // the newest code in the email is the one to type either way.
      await api.post('/auth/signup/resend', { signup_token: signup.token }, { auth: false });
      setResent(true);
      // A message about a code typed before would contradict the line under the heading.
      setCodeError(undefined);
    } catch (err) {
      setFormError(failure(err));
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
            Check your email
          </AppText>
        </View>
        <View style={{ marginTop: space.sm }}>
          <AppText role="body" tone="muted">
            {sentLine(signup.email, resent)}
          </AppText>
        </View>

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

        {formError ? (
          <View style={{ marginTop: space.md }} accessibilityLiveRegion="polite">
            <AppText role="body">{formError}</AppText>
          </View>
        ) : null}

        <View style={{ marginTop: space.lg }}>
          <PrimaryButton label="Verify code" onPress={verifyCode} loading={busy} disabled={busy} />
        </View>

        <Pressable
          onPress={() => void sendNewCode()}
          disabled={busy}
          accessibilityRole="button"
          hitSlop={8}
          style={{ marginTop: space.md, alignSelf: 'center', minHeight: 44, justifyContent: 'center' }}
        >
          <AppText role="button">Send a new code</AppText>
        </Pressable>
      </View>
    </Screen>
  );
}

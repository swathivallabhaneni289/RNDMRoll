import { useState } from 'react';
import { View } from 'react-native';
import { useRouter } from 'expo-router';
import { AppText, TextLink } from '@/components/ui/AppText';
import { PrimaryButton } from '@/components/ui/PrimaryButton';
import { Screen } from '@/components/ui/Screen';
import { TextField } from '@/components/ui/TextField';
import { space } from '@/lib/theme/tokens';
import { api, ApiError } from '@/lib/api/client';
import type { AuthResult } from '@/lib/api/types';
import { setDraft } from '@/lib/onboarding/draft';
import { useSession } from '@/lib/session/store';

/**
 * The one screen in this phase without an approved UI-SPEC entry (see
 * UI-SPEC revision 9's Checker Sign-Off recommendation and 01-07-PLAN.md
 * Task 3). Built from already-approved tokens/primitives; every string
 * introduced here beyond the Copywriting Contract is planner-authored and
 * provisional pending review at the plan 01-15 UAT checkpoint (see
 * 01-07-SUMMARY.md for the full list).
 *
 * Single screen, two modes toggled in place: the Copywriting Contract's
 * "Log in instead" / "Sign up instead" links are a same-screen toggle pair,
 * not two routes.
 */

type Mode = 'signup' | 'login';

const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

export default function EmailScreen() {
  const router = useRouter();
  const { signIn } = useSession();

  const [mode, setMode] = useState<Mode>('signup');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [emailError, setEmailError] = useState<string | undefined>();
  const [passwordError, setPasswordError] = useState<string | undefined>();
  const [formError, setFormError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  // Remount-triggered focus jump: TextField doesn't forward a ref, so an
  // imperative .focus() call isn't reachable from here. Bumping the key on
  // submit remounts the invalid field fresh with autoFocus set, which moves
  // the keyboard cursor there without needing TextField to expose a ref.
  const [attemptId, setAttemptId] = useState(0);
  const [autoFocusField, setAutoFocusField] = useState<'email' | 'password' | null>(null);

  function validateEmailValue(value: string): string | undefined {
    return EMAIL_PATTERN.test(value.trim()) ? undefined : 'Enter a valid email address.';
  }

  function validatePasswordValue(value: string): string | undefined {
    if (mode === 'signup') {
      return value.length >= 8 ? undefined : 'Password must be at least 8 characters.';
    }
    return value.length > 0 ? undefined : 'Enter your password.';
  }

  function toggleMode() {
    setMode((current) => (current === 'signup' ? 'login' : 'signup'));
    setEmailError(undefined);
    setPasswordError(undefined);
    setFormError(null);
  }

  async function handleSubmit() {
    const nextEmailError = validateEmailValue(email);
    const nextPasswordError = validatePasswordValue(password);
    setEmailError(nextEmailError);
    setPasswordError(nextPasswordError);

    if (nextEmailError || nextPasswordError) {
      setAutoFocusField(nextEmailError ? 'email' : 'password');
      setAttemptId((n) => n + 1);
      return;
    }

    setFormError(null);
    setSubmitting(true);
    try {
      if (mode === 'signup') {
        await api.post('/auth/signup', { email, password }, { auth: false });
        setDraft({ email, provider: 'email' });
        router.push('/verify-email');
      } else {
        const result = await api.post<AuthResult>('/auth/login', { email, password }, { auth: false });
        // Root layout's guard routes onward from here (profile-setup or (app)).
        await signIn(result);
      }
    } catch (err) {
      if (mode === 'signup' && err instanceof ApiError && err.code === 'email_taken') {
        setFormError("That email's already registered. Log in instead.");
        return;
      }
      if (mode === 'login' && err instanceof ApiError && err.code === 'email_not_verified') {
        // Next action is opening mail, not retrying the form. Not an error.
        setDraft({ email, provider: 'email' });
        router.push('/verify-email');
        return;
      }
      // A 401 invalid_credentials falls through to ApiError's own generic
      // fallback message here deliberately: the server refuses to
      // distinguish "wrong password" from "unknown account", so this screen
      // must render one message for both rather than inventing a second,
      // more specific one that would leak that distinction back to the UI.
      setFormError(err instanceof ApiError ? err.userMessage : 'Something went wrong. Please try again.');
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Screen>
      <View style={{ paddingTop: space.xxl }}>
        <AppText role="heading">{mode === 'signup' ? 'Sign up with email' : 'Log in'}</AppText>

        <View style={{ marginTop: space.lg }}>
          <TextField
            key={`email-${attemptId}`}
            label="Email"
            value={email}
            onChangeText={setEmail}
            onBlur={() => setEmailError(validateEmailValue(email))}
            error={emailError}
            keyboardType="email-address"
            autoCapitalize="none"
            autoComplete="email"
            textContentType="emailAddress"
            autoFocus={autoFocusField === 'email'}
          />
        </View>

        <View style={{ marginTop: space.md }}>
          <TextField
            key={`password-${attemptId}`}
            label="Password"
            value={password}
            onChangeText={setPassword}
            onBlur={() => setPasswordError(validatePasswordValue(password))}
            error={passwordError}
            secureTextEntry
            maxLength={72}
            autoComplete={mode === 'signup' ? 'new-password' : 'current-password'}
            textContentType={mode === 'signup' ? 'newPassword' : 'password'}
            autoFocus={autoFocusField === 'password'}
          />
          {mode === 'signup' ? (
            <View style={{ marginTop: space.xs }}>
              <AppText role="label" tone="muted">
                At least 8 characters.
              </AppText>
            </View>
          ) : null}
        </View>

        {formError ? (
          <View style={{ marginTop: space.md }}>
            <AppText role="body" tone="destructive">
              {formError}
            </AppText>
          </View>
        ) : null}

        <View style={{ marginTop: space.lg }}>
          <PrimaryButton
            label={mode === 'signup' ? 'Create account' : 'Log in'}
            onPress={handleSubmit}
            loading={submitting}
            disabled={submitting}
          />
        </View>

        <View style={{ marginTop: space.md, alignItems: 'center' }}>
          <TextLink onPress={toggleMode}>{mode === 'signup' ? 'Log in instead' : 'Sign up instead'}</TextLink>
        </View>
      </View>
    </Screen>
  );
}

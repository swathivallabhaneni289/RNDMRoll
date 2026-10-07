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
import { useSession } from '@/lib/session/store';

/**
 * Log in with email and password. Sign-up is its own page now (make-it-yours); the two
 * link to each other. An account that never verified its email logs in like any other.
 */

const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

export default function LoginScreen() {
  const router = useRouter();
  const { signIn } = useSession();

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

  // Only "not empty": a short wrong password must reach the server and come back as the
  // one "That email or password isn't right." message.
  function validatePasswordValue(value: string): string | undefined {
    return value.length > 0 ? undefined : 'Enter your password.';
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
      // A keyboard's trailing space passes client validation but the server rejects it.
      const result = await api.post<AuthResult>(
        '/auth/login',
        { email: email.trim(), password },
        { auth: false }
      );
      // The root guard routes onward from here: the app, or the finish page for an unfinished account.
      await signIn(result);
    } catch (err) {
      // One message for a wrong password and an unknown account (invalid_credentials): the
      // server refuses to tell them apart and so does this screen.
      if (err instanceof ApiError && err.code === 'validation_failed' && !err.field) {
        // The server's address check is stricter than this screen's: name the email, not "went wrong".
        setEmailError('Enter a valid email address.');
      } else {
        setFormError(err instanceof ApiError ? err.userMessage : 'Something went wrong. Please try again.');
      }
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Screen>
      <View style={{ paddingTop: space.xxl }}>
        <AppText role="heading">Log in</AppText>

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
            autoComplete="current-password"
            textContentType="password"
            autoFocus={autoFocusField === 'password'}
          />
        </View>

        {formError ? (
          <View style={{ marginTop: space.md }}>
            <AppText role="body" tone="destructive">
              {formError}
            </AppText>
          </View>
        ) : null}

        <View style={{ marginTop: space.lg }}>
          <PrimaryButton label="Log in" onPress={handleSubmit} loading={submitting} disabled={submitting} />
        </View>

        <View style={{ marginTop: space.md, alignItems: 'center' }}>
          <TextLink onPress={() => router.replace('/make-it-yours')}>Sign up instead</TextLink>
        </View>
      </View>
    </Screen>
  );
}

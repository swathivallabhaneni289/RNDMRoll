import { useState } from 'react';
import { Pressable, View } from 'react-native';
import { useRouter } from 'expo-router';
import { Ionicons } from '@expo/vector-icons';
import { AppText, TextLink } from '@/components/ui/AppText';
import { PrimaryButton } from '@/components/ui/PrimaryButton';
import { Screen } from '@/components/ui/Screen';
import { TextField } from '@/components/ui/TextField';
import { color, space } from '@/lib/theme/tokens';
import { api, ApiError } from '@/lib/api/client';
import type { AuthResult } from '@/lib/api/types';
import { useSession } from '@/lib/session/store';

/**
 * Log in with an email or a username, plus a password. Sign-up is its own page now
 * (make-it-yours). Back and "Sign up instead" both return to choose-method, where Email, Apple
 * and Google are offered, so a person who is not sure how they signed up can pick. An account
 * that never verified its email logs in like any other.
 */

const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

export default function LoginScreen() {
  const router = useRouter();
  const { signIn } = useSession();

  const [login, setLogin] = useState('');
  const [password, setPassword] = useState('');
  const [loginError, setLoginError] = useState<string | undefined>();
  const [passwordError, setPasswordError] = useState<string | undefined>();
  const [formError, setFormError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  // Remount-triggered focus jump: TextField doesn't forward a ref, so an
  // imperative .focus() call isn't reachable from here. Bumping the key on
  // submit remounts the invalid field fresh with autoFocus set, which moves
  // the keyboard cursor there without needing TextField to expose a ref.
  const [attemptId, setAttemptId] = useState(0);
  const [autoFocusField, setAutoFocusField] = useState<'login' | 'password' | null>(null);

  // This page is only pushed from choose-method (or swapped in for the sign-up form that was
  // pushed from it), so one step back is the method chooser. The replace is a safety net.
  function backToChooser() {
    if (router.canGoBack()) router.back();
    else router.replace('/choose-method');
  }

  // An @ means an email address, so it gets the address check. Anything else is taken as a
  // username and left to the server: a username that is not there gets the same "isn't right"
  // message as a wrong password.
  function validateLoginValue(value: string): string | undefined {
    const typed = value.trim();
    if (typed.length === 0) return 'Enter your email or username.';
    if (typed.includes('@') && !EMAIL_PATTERN.test(typed)) return 'Enter a valid email address.';
    return undefined;
  }

  // Only "not empty": a short wrong password must reach the server and come back as the
  // one "That email, username or password isn't right." message.
  function validatePasswordValue(value: string): string | undefined {
    return value.length > 0 ? undefined : 'Enter your password.';
  }

  async function handleSubmit() {
    const nextLoginError = validateLoginValue(login);
    const nextPasswordError = validatePasswordValue(password);
    setLoginError(nextLoginError);
    setPasswordError(nextPasswordError);

    if (nextLoginError || nextPasswordError) {
      setAutoFocusField(nextLoginError ? 'login' : 'password');
      setAttemptId((n) => n + 1);
      return;
    }

    setFormError(null);
    setSubmitting(true);
    try {
      // A keyboard's trailing space passes client validation but the server rejects it.
      const result = await api.post<AuthResult>(
        '/auth/login',
        { login: login.trim(), password },
        { auth: false }
      );
      // The root guard routes onward from here: the app, or the finish page for an unfinished account.
      await signIn(result);
    } catch (err) {
      // One message for a wrong password, an unknown email and an unknown username
      // (invalid_credentials): the server refuses to tell them apart and so does this screen.
      if (err instanceof ApiError && err.code === 'validation_failed') {
        // The server's address check is stricter than this screen's: name the box, not "went wrong".
        setLoginError(login.includes('@') ? 'Enter a valid email address.' : 'Enter your email or username.');
      } else {
        setFormError(err instanceof ApiError ? err.userMessage : 'Something went wrong. Please try again.');
      }
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Screen>
      <View style={{ paddingTop: space.lg }}>
        <Pressable
          onPress={backToChooser}
          accessibilityRole="button"
          accessibilityLabel="Back"
          hitSlop={12}
          style={{ flexDirection: 'row', alignItems: 'center', alignSelf: 'flex-start' }}
        >
          <Ionicons name="chevron-back" size={22} color={color.ink} />
          <AppText role="body">Back</AppText>
        </Pressable>

        <View style={{ marginTop: space.xl }}>
          <AppText role="heading">Log in</AppText>
        </View>

        <View style={{ marginTop: space.lg }}>
          <TextField
            key={`login-${attemptId}`}
            label="Email or username"
            value={login}
            onChangeText={setLogin}
            onBlur={() => setLoginError(validateLoginValue(login))}
            error={loginError}
            keyboardType="email-address"
            autoCapitalize="none"
            autoComplete="username"
            textContentType="username"
            autoFocus={autoFocusField === 'login'}
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
          <TextLink onPress={backToChooser}>Sign up instead</TextLink>
        </View>
      </View>
    </Screen>
  );
}

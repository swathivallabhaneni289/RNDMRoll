import { useEffect, useRef, useState } from 'react';
import { View } from 'react-native';
import * as Linking from 'expo-linking';
import { Ionicons } from '@expo/vector-icons';
import { AppText } from '@/components/ui/AppText';
import { IconBadge } from '@/components/ui/IconBadge';
import { Screen } from '@/components/ui/Screen';
import { TextButton } from '@/components/ui/TextButton';
import { color, space } from '@/lib/theme/tokens';
import { api, ApiError } from '@/lib/api/client';
import type { AuthResult } from '@/lib/api/types';
import { useDraft } from '@/lib/onboarding/draft';
import { useSession } from '@/lib/session/store';

/**
 * D-05 step 2 / D-04's verification gate: the waiting screen a user lands on
 * after email signup (or an unverified login attempt). Unchanged from
 * UI-SPEC revision 8: the phase's second and final Display promotion
 * ("Check your email"), no background texture.
 */

const RESEND_COOLDOWN_SECONDS = 30;
const SUCCESS_ADVANCE_DELAY_MS = 900;

type VerifyStatus = 'pending' | 'verifying' | 'success' | 'expired' | 'consumed' | 'error';

function extractToken(url: string): string | undefined {
  const parsed = Linking.parse(url);
  const value = parsed.queryParams?.token;
  if (typeof value === 'string') return value;
  if (Array.isArray(value)) return value[0];
  return undefined;
}

export default function VerifyEmailScreen() {
  const draft = useDraft();
  const { signIn } = useSession();
  const email = draft.email ?? '';

  const [status, setStatus] = useState<VerifyStatus>('pending');
  const [pendingAuthResult, setPendingAuthResult] = useState<AuthResult | null>(null);
  const [cooldown, setCooldown] = useState(0);
  const [resending, setResending] = useState(false);
  const processedTokenRef = useRef<string | null>(null);

  async function verifyToken(token: string) {
    setStatus('verifying');
    try {
      const result = await api.post<AuthResult>('/auth/verify-email', { token }, { auth: false });
      setPendingAuthResult(result);
      setStatus('success');
    } catch (err) {
      if (err instanceof ApiError && err.code === 'token_expired') {
        setStatus('expired');
      } else if (err instanceof ApiError && err.code === 'token_consumed') {
        setStatus('consumed');
      } else {
        setStatus('error');
      }
    }
  }

  function handleIncomingUrl(url: string) {
    const token = extractToken(url);
    if (!token || processedTokenRef.current === token) return;
    processedTokenRef.current = token;
    verifyToken(token);
  }

  // Live deep-link return while the app is already running.
  useEffect(() => {
    const subscription = Linking.addEventListener('url', (event) => handleIncomingUrl(event.url));
    return () => subscription.remove();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Cold start: the app was launched directly by the verification link.
  useEffect(() => {
    Linking.getInitialURL().then((url) => {
      if (url) handleIncomingUrl(url);
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Brief confirmation moment before handing off, per UI-SPEC: a visible
  // "Verified" state, then automatic advance — no manual Continue tap, no
  // silent jump straight into the app.
  useEffect(() => {
    if (status !== 'success' || !pendingAuthResult) return;
    const timeout = setTimeout(() => {
      signIn(pendingAuthResult);
    }, SUCCESS_ADVANCE_DELAY_MS);
    return () => clearTimeout(timeout);
  }, [status, pendingAuthResult, signIn]);

  useEffect(() => {
    if (cooldown <= 0) return;
    const interval = setInterval(() => {
      setCooldown((seconds) => (seconds <= 1 ? 0 : seconds - 1));
    }, 1000);
    return () => clearInterval(interval);
  }, [cooldown]);

  async function handleResend() {
    if (cooldown > 0 || resending) return;
    setResending(true);
    try {
      await api.post('/auth/verify-email/resend', { email }, { auth: false });
    } catch {
      // Best-effort: UI-SPEC's stated resend behavior returns 202 regardless
      // of whether the account exists, so a network hiccup here shouldn't
      // block the cooldown from starting (retrying instantly would just hit
      // the same server-side rate limit).
    } finally {
      setResending(false);
      setCooldown(RESEND_COOLDOWN_SECONDS);
    }
  }

  const resendDisabled = cooldown > 0 || resending;
  const resendAccessibilityLabel =
    cooldown > 0 ? `Resend email, available in ${cooldown} seconds` : 'Resend email';

  return (
    <Screen>
      <View style={{ flex: 1, justifyContent: 'center', alignItems: 'center' }}>
        {status === 'success' ? (
          <>
            <Ionicons name="checkmark-circle" size={40} color={color.success} />
            <View style={{ marginTop: space.md }}>
              <AppText role="heading">Verified</AppText>
            </View>
          </>
        ) : (
          <>
            <IconBadge name="mail-outline" surface="dominant" accessibilityLabel="Verification email sent" />

            <View style={{ marginTop: space.lg }}>
              <AppText role="display">Check your email</AppText>
            </View>

            <View style={{ marginTop: space.md, paddingHorizontal: space.lg }}>
              <AppText role="body">{`We sent a verification link to ${email}. Tap it to continue.`}</AppText>
            </View>

            {status === 'expired' || status === 'consumed' ? (
              <View style={{ marginTop: space.md, paddingHorizontal: space.lg }}>
                <AppText role="body" tone="destructive">
                  {status === 'expired'
                    ? 'That link expired. Send a new one below.'
                    : 'That link was already used. Send a new one below.'}
                </AppText>
              </View>
            ) : null}

            {status === 'error' ? (
              <View style={{ marginTop: space.md, paddingHorizontal: space.lg }}>
                <AppText role="body" tone="destructive">
                  Something went wrong. Please try again.
                </AppText>
              </View>
            ) : null}

            <View
              style={{ marginTop: space.lg }}
              accessible
              accessibilityRole="button"
              accessibilityLabel={resendAccessibilityLabel}
              accessibilityState={{ disabled: resendDisabled }}
            >
              <TextButton label="Resend email" onPress={handleResend} disabled={resendDisabled} />
            </View>
          </>
        )}
      </View>
    </Screen>
  );
}

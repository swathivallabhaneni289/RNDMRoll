import * as AppleAuthentication from 'expo-apple-authentication';
import { api } from '@/lib/api/client';
import type { AuthResult } from '@/lib/api/types';
import { setDraft } from '@/lib/onboarding/draft';

/**
 * Native Apple and Google sign-in wrappers. Both post only the raw provider
 * identity token to the backend oauth endpoints (POST /v1/auth/oauth/apple,
 * POST /v1/auth/oauth/google) — never a client-shaped identity payload.
 * RESEARCH.md Pitfall 3 and PATTERNS.md's "Server-side token verification"
 * pattern both require the backend to independently re-verify signature,
 * issuer, audience, and expiry (plan 01-10); a client-asserted email or
 * provider field would let a modified client claim any address, so neither
 * function ever sends one. `full_name` on the Apple call is the one
 * exception: it rides along as untrusted display text for prefill
 * convenience, not as an identity claim.
 */

/**
 * Returned instead of throwing when the user backs out of the native
 * sign-in sheet, so the calling screen can distinguish a deliberate cancel
 * (return silently to idle, no error) from a real failure (show the error
 * copy). UI-SPEC's social sign-in contract requires exactly this split.
 */
export const SIGN_IN_CANCELED = Symbol('social-sign-in-canceled');

export type SocialSignInResult = AuthResult | typeof SIGN_IN_CANCELED;

export function isAppleSignInAvailable(): Promise<boolean> {
  return AppleAuthentication.isAvailableAsync();
}

function isRequestCanceledError(error: unknown): boolean {
  const code = (error as { code?: unknown } | null)?.code;
  return code === 'ERR_REQUEST_CANCELED';
}

function joinFullName(fullName: AppleAuthentication.AppleAuthenticationFullName | null): string | null {
  if (!fullName) return null;
  const parts = [fullName.givenName, fullName.familyName].filter(
    (part): part is string => typeof part === 'string' && part.length > 0
  );
  return parts.length > 0 ? parts.join(' ') : null;
}

/**
 * Apple returns `fullName`/`email` only on a user's very first authorization
 * for this Apple ID + bundle ID (RESEARCH.md Pitfall 1b) — every later call
 * returns null for both. Whatever comes back this call is captured into the
 * onboarding draft immediately, since a second attempt would come back empty.
 */
export async function signInWithApple(): Promise<SocialSignInResult> {
  let credential: AppleAuthentication.AppleAuthenticationCredential;
  try {
    credential = await AppleAuthentication.signInAsync({
      requestedScopes: [
        AppleAuthentication.AppleAuthenticationScope.FULL_NAME,
        AppleAuthentication.AppleAuthenticationScope.EMAIL,
      ],
    });
  } catch (error) {
    if (isRequestCanceledError(error)) {
      return SIGN_IN_CANCELED;
    }
    throw error;
  }

  const fullName = joinFullName(credential.fullName);

  const result = await api.post<AuthResult>(
    '/auth/oauth/apple',
    { identity_token: credential.identityToken, full_name: fullName },
    { auth: false }
  );

  setDraft({
    provider: 'apple',
    ...(fullName ? { suggestedName: fullName } : {}),
    ...(credential.email ? { email: credential.email } : {}),
  });

  return result;
}

let googleSigninConfigured = false;

/**
 * The Google Sign-In package's own module-level code calls
 * TurboModuleRegistry.getEnforcing() as soon as it is required, which throws
 * immediately anywhere the native module isn't compiled in (Expo Go, web —
 * this library has no web implementation at all). A static top-level import
 * in this file previously made that throw happen the moment choose-method.tsx
 * loaded this module, taking down the whole (auth) navigation stack (every
 * screen it declares) rather than just the Google button. Deferring the
 * import to first actual use means every other screen and sign-in method
 * keeps working on platforms without the native module; only an actual tap
 * on "Continue with Google" fails there, which is the correct, contained
 * failure surface.
 */
async function getGoogleSignin() {
  const { GoogleSignin } = await import('@react-native-google-signin/google-signin');
  if (!googleSigninConfigured) {
    // SDK-resolved client IDs come from the two EXPO_PUBLIC_* env keys this
    // module introduces. Safe to call with undefined values before those are
    // provisioned in .env — configure() itself does not throw on undefined,
    // only a later hasPlayServices()/signIn() call does.
    GoogleSignin.configure({
      iosClientId: process.env.EXPO_PUBLIC_GOOGLE_CLIENT_ID_IOS,
      webClientId: process.env.EXPO_PUBLIC_GOOGLE_CLIENT_ID_WEB,
    });
    googleSigninConfigured = true;
  }
  return GoogleSignin;
}

export async function signInWithGoogle(): Promise<SocialSignInResult> {
  const GoogleSignin = await getGoogleSignin();
  await GoogleSignin.hasPlayServices();
  const response = await GoogleSignin.signIn();

  if (response.type === 'cancelled') {
    return SIGN_IN_CANCELED;
  }

  const idToken = response.data.idToken;
  if (!idToken) {
    throw new Error('Google sign-in did not return an ID token');
  }

  const result = await api.post<AuthResult>('/auth/oauth/google', { id_token: idToken }, { auth: false });

  setDraft({ provider: 'google' });

  return result;
}

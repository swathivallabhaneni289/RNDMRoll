import * as SecureStore from 'expo-secure-store';
import {
  createContext,
  createElement,
  useCallback,
  useContext,
  useEffect,
  useState,
  type ReactNode,
} from 'react';
import { api } from '@/lib/api/client';
import type { ApiUser, AuthResult } from '@/lib/api/types';

/**
 * SecureStore-backed session persistence and React context. This is the only module
 * in the repository permitted to import expo-secure-store: every credential (access
 * and refresh token) is written to and read from the OS keychain here, and nowhere
 * else, including React state that survives a reload or any logging call.
 */

const ACCESS_TOKEN_KEY = 'rndmroll.access_token';
const REFRESH_TOKEN_KEY = 'rndmroll.refresh_token';

const KEYCHAIN_OPTIONS: SecureStore.SecureStoreOptions = {
  keychainAccessible: SecureStore.WHEN_UNLOCKED_THIS_DEVICE_ONLY,
};

export type SessionStatus = 'loading' | 'unauthenticated' | 'authenticated';

// ---------------------------------------------------------------------------
// Module-level accessors lib/api/client.ts imports. These exist so the fetch
// client can attach a bearer token and drive refresh/sign-out without ever
// importing React context or touching SecureStore itself.
// ---------------------------------------------------------------------------

let inMemoryAccessToken: string | null = null;

export function getAccessToken(): string | null {
  return inMemoryAccessToken;
}

export function setAccessToken(token: string | null): void {
  inMemoryAccessToken = token;
}

interface SessionHandlers {
  refreshSession: () => Promise<void>;
  signOut: () => Promise<void>;
}

let registeredHandlers: SessionHandlers | null = null;

/**
 * Called once by SessionProvider on mount, registering the live, state-updating
 * implementations of refresh/sign-out so the module-level functions below (which
 * lib/api/client.ts calls) can drive them without importing React context.
 */
export function registerSessionHandlers(handlers: SessionHandlers): void {
  registeredHandlers = handlers;
}

/**
 * Delegate lib/api/client.ts calls on a 401. Throws if called before SessionProvider
 * has mounted, which should never happen in practice since it wraps the whole app.
 */
export async function refreshSession(): Promise<void> {
  if (!registeredHandlers) {
    throw new Error('refreshSession called before SessionProvider mounted');
  }
  return registeredHandlers.refreshSession();
}

export async function signOut(): Promise<void> {
  if (!registeredHandlers) {
    throw new Error('signOut called before SessionProvider mounted');
  }
  return registeredHandlers.signOut();
}

// ---------------------------------------------------------------------------
// React context
// ---------------------------------------------------------------------------

interface SessionContextValue {
  status: SessionStatus;
  user: ApiUser | null;
  signIn: (result: AuthResult) => Promise<void>;
  signOut: () => Promise<void>;
  refreshSession: () => Promise<void>;
  reloadUser: () => Promise<void>;
}

const SessionContext = createContext<SessionContextValue | undefined>(undefined);

export function SessionProvider({ children }: { children: ReactNode }) {
  const [status, setStatus] = useState<SessionStatus>('loading');
  const [user, setUser] = useState<ApiUser | null>(null);

  const signIn = useCallback(async (result: AuthResult) => {
    await SecureStore.setItemAsync(ACCESS_TOKEN_KEY, result.access_token, KEYCHAIN_OPTIONS);
    await SecureStore.setItemAsync(REFRESH_TOKEN_KEY, result.refresh_token, KEYCHAIN_OPTIONS);
    setAccessToken(result.access_token);
    setUser(result.user);
    setStatus('authenticated');
  }, []);

  const doSignOut = useCallback(async () => {
    try {
      const storedRefreshToken = await SecureStore.getItemAsync(REFRESH_TOKEN_KEY);
      if (storedRefreshToken) {
        await api.post('/auth/logout', { refresh_token: storedRefreshToken }, { auth: false });
      }
    } catch {
      // Best-effort: a network failure here must not trap the user in a signed-in shell.
    }
    await SecureStore.deleteItemAsync(ACCESS_TOKEN_KEY);
    await SecureStore.deleteItemAsync(REFRESH_TOKEN_KEY);
    setAccessToken(null);
    setUser(null);
    setStatus('unauthenticated');
  }, []);

  const doRefreshSession = useCallback(async () => {
    try {
      const storedRefreshToken = await SecureStore.getItemAsync(REFRESH_TOKEN_KEY);
      if (!storedRefreshToken) {
        throw new Error('no refresh token stored');
      }
      // The server rotates the refresh token on every call, so both returned tokens
      // must be persisted, not only the access token, or the next launch is stranded.
      const result = await api.post<AuthResult>(
        '/auth/refresh',
        { refresh_token: storedRefreshToken },
        { auth: false }
      );
      await SecureStore.setItemAsync(ACCESS_TOKEN_KEY, result.access_token, KEYCHAIN_OPTIONS);
      await SecureStore.setItemAsync(REFRESH_TOKEN_KEY, result.refresh_token, KEYCHAIN_OPTIONS);
      setAccessToken(result.access_token);
    } catch (err) {
      await doSignOut();
      throw err;
    }
  }, [doSignOut]);

  const reloadUser = useCallback(async () => {
    const freshUser = await api.get<ApiUser>('/me');
    setUser(freshUser);
  }, []);

  useEffect(() => {
    registerSessionHandlers({ refreshSession: doRefreshSession, signOut: doSignOut });
  }, [doRefreshSession, doSignOut]);

  // Runs exactly once on mount: this is the relaunch half of ACCT-01's
  // stay-logged-in chain. Absence of a refresh token means a genuinely signed-out
  // device; presence means attempt to restore, falling back to unauthenticated
  // on any failure (expired/revoked refresh token, network error at launch, etc).
  useEffect(() => {
    let cancelled = false;

    async function restore() {
      const storedRefreshToken = await SecureStore.getItemAsync(REFRESH_TOKEN_KEY);
      if (!storedRefreshToken) {
        if (!cancelled) setStatus('unauthenticated');
        return;
      }
      try {
        await doRefreshSession();
        await reloadUser();
        if (!cancelled) setStatus('authenticated');
      } catch {
        if (!cancelled) setStatus('unauthenticated');
      }
    }

    restore();

    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const value: SessionContextValue = {
    status,
    user,
    signIn,
    signOut: doSignOut,
    refreshSession: doRefreshSession,
    reloadUser,
  };

  return createElement(SessionContext.Provider, { value }, children);
}

export function useSession(): SessionContextValue {
  const ctx = useContext(SessionContext);
  if (!ctx) {
    throw new Error('useSession must be used within a SessionProvider');
  }
  return ctx;
}

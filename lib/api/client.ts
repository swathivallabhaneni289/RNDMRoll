import { getAccessToken, refreshSession, signOut } from '@/lib/session/store';
import type { ApiErrorBody, ApiErrorCode } from '@/lib/api/types';

/**
 * Typed fetch client every mobile screen calls through. Attaches the bearer token,
 * converts non-2xx responses into a typed ApiError, and single-flights a refresh+retry
 * on a 401 token_expired response so a burst of concurrent expired-token requests
 * produces exactly one refresh (see the module-level `refreshPromise` below).
 *
 * This module never reads or writes the OS keychain directly. It reaches session
 * state only through `getAccessToken` (an in-memory accessor) and the
 * `refreshSession`/`signOut` delegates that lib/session/store.ts exports;
 * lib/session/store.ts is the only module in the repo permitted to import the
 * secure-storage package.
 */

const BASE_URL = process.env.EXPO_PUBLIC_API_BASE_URL ?? 'http://localhost:8080';
const API_PREFIX = '/v1';

/**
 * UI-SPEC's Copywriting Contract error strings, verbatim, for the codes it declares.
 * Any other code falls through to a generic message the calling screen is expected
 * to override with screen-specific copy.
 */
const USER_MESSAGES: Partial<Record<ApiErrorCode, string>> = {
  network_unavailable: "Couldn't connect. Check your connection and try again.",
  email_taken: "That email's already registered. Log in instead.",
};

const GENERIC_FALLBACK_MESSAGE = 'Something went wrong. Please try again.';

export class ApiError extends Error {
  status: number;
  code: ApiErrorCode;
  suggestions?: string[];

  constructor(status: number, code: ApiErrorCode, message?: string, suggestions?: string[]) {
    super(message ?? code);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
    this.suggestions = suggestions;
  }

  get userMessage(): string {
    return USER_MESSAGES[this.code] ?? GENERIC_FALLBACK_MESSAGE;
  }
}

interface RequestOptions {
  /** Set false to omit the Authorization header (login, signup, refresh itself, etc). */
  auth?: boolean;
  /** Internal: set true on the single retry attempt after a refresh, to prevent looping. */
  retry?: boolean;
}

/**
 * Single-flight guard: several requests failing with an expired token concurrently
 * must await one shared refresh rather than each firing its own, which would rotate
 * the server-side refresh token repeatedly and strand the other callers.
 */
let refreshPromise: Promise<void> | null = null;

async function request<T>(
  method: 'GET' | 'POST' | 'PATCH',
  path: string,
  body: unknown,
  opts: RequestOptions
): Promise<T> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
  };

  if (opts.auth !== false) {
    const token = getAccessToken();
    if (token) {
      headers.Authorization = `Bearer ${token}`;
    }
  }

  let response: Response;
  try {
    response = await fetch(`${BASE_URL}${API_PREFIX}${path}`, {
      method,
      headers,
      body: body !== undefined ? JSON.stringify(body) : undefined,
    });
  } catch {
    throw new ApiError(0, 'network_unavailable');
  }

  if (response.ok) {
    if (response.status === 204) {
      return undefined as T;
    }
    return (await response.json()) as T;
  }

  let errorBody: ApiErrorBody;
  try {
    errorBody = (await response.json()) as ApiErrorBody;
  } catch {
    throw new ApiError(response.status, 'server_error');
  }

  const code = errorBody.error as ApiErrorCode;
  const apiError = new ApiError(response.status, code, errorBody.message, errorBody.suggestions);

  if (response.status === 401 && code === 'token_expired' && !opts.retry) {
    if (!refreshPromise) {
      refreshPromise = refreshSession().finally(() => {
        refreshPromise = null;
      });
    }
    try {
      await refreshPromise;
    } catch {
      await signOut();
      throw apiError;
    }
    return request<T>(method, path, body, { ...opts, retry: true });
  }

  throw apiError;
}

export const api = {
  get<T>(path: string, opts: RequestOptions = {}): Promise<T> {
    return request<T>('GET', path, undefined, opts);
  },
  post<T>(path: string, body: unknown, opts: RequestOptions = {}): Promise<T> {
    return request<T>('POST', path, body, opts);
  },
  patch<T>(path: string, body: unknown, opts: RequestOptions = {}): Promise<T> {
    return request<T>('PATCH', path, body, opts);
  },
};

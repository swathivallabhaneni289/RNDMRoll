/**
 * Wire contract shared by every mobile screen. This is the single source of truth
 * for the shapes the Go backend (plans 01-08 through 01-11) returns and accepts.
 * No screen or hook re-declares any of these types locally.
 */

export interface ApiUser {
  id: string;
  email: string;
  name: string | null;
  username: string | null;
  bio: string | null;
  avatar_url: string | null;
  email_verified: boolean;
  onboarding_complete: boolean;
}

export interface SessionTokens {
  access_token: string;
  refresh_token: string;
  expires_in: number;
}

/**
 * The session body POST /auth/login, /auth/signup and the oauth endpoints return.
 * /auth/refresh returns only the three token fields (SessionTokens), no user.
 * No user object anywhere carries a birthday.
 */
export type AuthResult = SessionTokens & {
  user: ApiUser;
  is_new_user?: boolean;
};

export interface UsernameSuggestion {
  username: string;
  alternates: string[];
}

export interface UsernameAvailability {
  available: boolean;
  alternates: string[];
}

export interface AvatarUploadTicket {
  upload_url: string;
  public_url: string;
  expires_in: number;
}

export interface ApiErrorBody {
  error: string;
  message?: string;
  suggestions?: string[];
  /** Set on a 400 validation_failed: which form field the server rejected. */
  field?: string;
}

/**
 * Every error code the backend can return in an `ApiErrorBody.error` field.
 * `lib/api/client.ts` uses this union to type `ApiError.code`. `network_unavailable`,
 * `photo_rejected` and `photo_upload_failed` are raised by the app itself, never sent by the server.
 */
export type ApiErrorCode =
  | 'invalid_credentials'
  | 'email_taken'
  | 'username_taken'
  | 'under_minimum_age'
  | 'payload_too_large'
  | 'token_invalid'
  | 'not_found'
  | 'subject_linked'
  | 'token_expired'
  | 'token_consumed'
  | 'rate_limited'
  | 'validation_failed'
  | 'network_unavailable'
  | 'photo_rejected'
  | 'photo_upload_failed'
  | 'server_error';

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

export type AuthResult = SessionTokens & {
  user: ApiUser;
  is_new_user?: boolean;
};

export interface SignupResult {
  user_id: string;
}

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
}

/**
 * Every error code the backend can return in an `ApiErrorBody.error` field.
 * `lib/api/client.ts` uses this union to type `ApiError.code`.
 */
export type ApiErrorCode =
  | 'invalid_credentials'
  | 'email_taken'
  | 'username_taken'
  | 'email_not_verified'
  | 'token_invalid'
  | 'token_expired'
  | 'token_consumed'
  | 'rate_limited'
  | 'validation_failed'
  | 'network_unavailable'
  | 'server_error';

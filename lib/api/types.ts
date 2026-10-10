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
 * The session body POST /auth/login and /auth/signup return.
 * /auth/refresh returns only the three token fields (SessionTokens), no user.
 * No user object anywhere carries a birthday.
 */
export type AuthResult = SessionTokens & {
  user: ApiUser;
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
  /** Set on a 400 wheel_invalid: a fixed word for what is wrong with the list. */
  reason?: string;
  /** Set on a 400 wheel_invalid: which row of the sent list is wrong, counted from 0. */
  index?: number;
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
  | 'token_expired'
  | 'token_consumed'
  | 'rate_limited'
  | 'validation_failed'
  | 'network_unavailable'
  | 'photo_rejected'
  | 'photo_upload_failed'
  | 'server_error'
  // Phase 2 (the daily spin and the wheel; sent by the server, plan 02-01 Task 3a).
  | 'spin_not_open'
  | 'spin_closed'
  | 'no_bonus'
  | 'day_settled'
  | 'wheel_stale'
  | 'wheel_invalid'
  | 'profile_incomplete';

// ---------------------------------------------------------------------------
// Phase 2: the daily spin and the wheel (plan 02-01 Task 3b and 3c, the exact wire shapes).
// Instants are RFC 3339 UTC strings. The phone shows what the server says: the date, the
// window, the winner, the time a spin was made (`spun_local_time`) and the next opening's
// weekday name (`next_open_weekday`) all come written by the server.
// ---------------------------------------------------------------------------

/**
 * `waiting`, `open`, `result` and `late` are what Phase 2 sends. `posted` and `skipped` are
 * Phase 3's; the type is kept open to them so Phase 3 adds states without editing this file.
 */
export type TodayState = 'waiting' | 'open' | 'result' | 'late' | 'posted' | 'skipped';

/** One category on a wheel as the spin and Today carry it (a snapshot of a wheel). */
export interface WheelEntry {
  id: string;
  name: string;
  weight: number;
}

export interface Spin {
  id: string;
  /** The civil date the spin belongs to, "2026-10-08". */
  local_date: string;
  kind: 'daily' | 'bonus';
  /** 0 for the day's first spin, then 1, 2 ... for a bonus spin. */
  seq: number;
  category: { id: string; name: string };
  /** Index into `wheel` of the winning wedge. */
  winner_index: number;
  /** The wheel this spin used, in order. */
  wheel: WheelEntry[];
  spun_at: string;
  /** "8:02 PM", written by the server in the zone the spin was made in. Printed as sent. */
  spun_local_time: string;
  result: 'open' | 'posted' | 'skipped';
}

export interface WeekDay {
  /** "2026-10-05". */
  date: string;
  /** "M", "T", "W" ... from the server. */
  letter: string;
  /** "Monday" ... from the server. */
  weekday: string;
  posted: boolean;
  /** Phase 3 only. */
  skipped?: boolean;
}

export interface Week {
  /** The Monday the week starts on. */
  start: string;
  goal: number;
  posted: number;
  goal_met: boolean;
  /** Index (0 to 6, Monday first) of today inside `days`. */
  today_index: number;
  days: WeekDay[];
}

export interface Bonus {
  /** A bonus spin is held. */
  held: boolean;
  /** It can be used right now (state `result`, the day not settled, one held). */
  can_use: boolean;
}

export interface Today {
  now: string;
  time_zone: string;
  /** The person's civil date, "2026-10-08". */
  local_date: string;
  state: TodayState;
  window: { opens_at: string; closes_at: string };
  next_open_at: string;
  /** "Friday", written by the server. Printed as sent. */
  next_open_weekday: string;
  spin: Spin | null;
  /** The spin's own wheel when a spin is shown, else the live wheel. */
  wheel: WheelEntry[];
  week: Week;
  bonus: Bonus;
}

export interface SpinResponse {
  /** True when the server already had this spin (a retry, or another phone). */
  already: boolean;
  today: Today;
}

export interface WheelCategory {
  id: string;
  name: string;
  weight: number;
  position: number;
}

export interface WheelLimits {
  min: number;
  max: number;
  name_max: number;
  weight_min: number;
  weight_max: number;
}

export interface WheelResponse {
  version: number;
  categories: WheelCategory[];
  limits: WheelLimits;
}

/** One row of the list "Save wheel" sends. `id` is null for a category added in this visit. */
export interface WheelSaveEntry {
  id: string | null;
  name: string;
  weight: number;
}

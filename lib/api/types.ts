/**
 * Wire contract shared by every mobile screen. This is the single source of truth
 * for the shapes the Go backend (plans 01-08 through 01-11) returns and accepts.
 * No screen or hook re-declares any of these types locally.
 */

import type { ReactionName } from '@/lib/entry/reactions';

/**
 * Whether an account can appear on Global (Phase 2.1, migration 0005). 'public' is the default for
 * everyone; the Settings switch "Private account" is ON for 'private'. Only stored and shown for now.
 */
export type AccountVisibility = 'public' | 'private';

export interface ApiUser {
  id: string;
  email: string;
  name: string | null;
  username: string | null;
  bio: string | null;
  avatar_url: string | null;
  email_verified: boolean;
  onboarding_complete: boolean;
  account_visibility: AccountVisibility;
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
  /**
   * True on a log in or a Forgot password reset that cancelled the account's pending deletion (it
   * was asked for less than 30 days before; POST /me/delete). Left out otherwise, so read it with
   * `=== true`.
   */
  restored?: boolean;
};

/**
 * The answer of POST /me/delete (developer, 2026-10-25: delete the account like Instagram). The
 * account is hidden from everyone at once and deleted for good after 30 days.
 */
export interface AccountDeletion {
  /** When the account is deleted for good, an RFC 3339 instant. */
  delete_after: string;
  /**
   * That day, written by the server in the person's time zone ("Tuesday, November 24"). Printed
   * as sent: the phone never formats a date itself.
   */
  delete_on_label: string;
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
  /**
   * Where the photo is stored, "avatars/<user id>/<16 hex>.<ext>" (private photos, 2026-11-09).
   * PATCH /me saves it as `avatar_key`; the user's `avatar_url` is then a signed link to it.
   */
  avatar_key: string;
  expires_in: number;
}

export interface ApiErrorBody {
  error: string;
  message?: string;
  suggestions?: string[];
  /** Set on a 400 validation_failed: which form field the server rejected. */
  field?: string;
  /**
   * Set on a 400 wheel_invalid: a fixed word for what is wrong with the list. Also on the 400 of
   * POST /auth/password/verify and POST /auth/password/reset with field "code": "wrong_code" (a
   * wrong, expired or used-up code, a code a reset already spent, or an email with no account or a
   * disabled one: the same answer for each), "replaced" (the code the newest code replaced; no try
   * is counted), "used" (reset only: the right code, but another request used it first; no try is
   * counted) or "invalid" (not 6 digits). And on the 400 of POST /me/delete with field "password":
   * "incorrect" (a wrong password) or "required".
   */
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
  | 'account_disabled'
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
  | 'profile_incomplete'
  // Phase 3 (entries and skips; sent by the server, plan 03-01 Task 3a). `day_settled` is Phase 2's.
  | 'no_spin'
  | 'day_closed'
  | 'spin_changed'
  // Phase 5 (follows, the Board, reactions and comments; 05-FOLLOW-OVERRIDES section 6).
  | 'own_post'
  | 'comments_off'
  // A second look at a removed post or comment (developer, 2026-11-19), each a 409 of
  // POST /removals/{id}/second-look: asked already, past the removal's 30 days, or answered by a
  // Bring it back (lib/notifications/moderation.ts words them).
  | 'second_look_asked'
  | 'second_look_too_late'
  | 'second_look_back';

// ---------------------------------------------------------------------------
// Phase 2: the daily spin and the wheel (plan 02-01 Task 3b and 3c, the exact wire shapes).
// Instants are RFC 3339 UTC strings. The phone shows what the server says: the date, the
// window, the winner, the time a spin was made (`spun_local_time`) and the next opening's
// weekday name (`next_open_weekday`) all come written by the server.
// ---------------------------------------------------------------------------

/**
 * `waiting`, `open`, `result` and `late` are what Phase 2 sends. `posted` and `skipped` are
 * Phase 3's; the type is kept open to them so Phase 3 adds states without editing this file.
 * `late` means it is past midnight and the spin made before it is still unsettled. The page still
 * calls it today's spin (a spin's day runs from 8 PM to the next 8 PM), and whether a post made now
 * counts as late is `Today.post_late`, not this state.
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
  /**
   * Phase 3: the day's spin was skipped (and the day has no post). A Phase 2 server leaves the key
   * out, so read it as `day.skipped === true`.
   */
  skipped: boolean;
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

/**
 * The summary of the entry of the shown spin's date that Today carries (plan 03-01 Task 3b), so
 * the Posted row draws with no second request. `stars` is the stored whole number 1 to 10 (7 means
 * 3.5 stars), `photo_url` is the cover photo's link and `photo_count` is 1 to 10.
 */
export interface TodayEntry {
  id: string;
  late: boolean;
  title: string;
  stars: number;
  reaction: ReactionName | null;
  photo_url: string;
  photo_count: number;
  /** Archived by the owner: the day still counts, and the row says so instead of showing the entry. */
  archived: boolean;
  comments_off: boolean;
  /**
   * 3 or more reports wait for the admin, so no one else sees the entry until then. Left out
   * otherwise, so read it with `=== true`.
   */
  under_review?: boolean;
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
  /**
   * The instant from which a post for the shown spin counts as late: noon of the day after the
   * spin's date, in the person's zone. Null unless a spin is shown and its day is not settled
   * (state Result or Late).
   */
  late_from: string | null;
  /**
   * A post made now would count as late (the server's now is at or after `late_from`). An older
   * server leaves the key out, so read it with `=== true`.
   */
  post_late: boolean;
  /**
   * The spin of the date before, when its day closed (the next 8 PM) with no post and no skip: its
   * category and the server's weekday, for "Monday's movie got away." Null in every state but
   * Open and Waiting. An older server leaves the key out, so read it with `?? null`.
   */
  missed: { category: string; weekday: string } | null;
  /**
   * Waiting, after midnight: the evening before had no spin at all, so the page says "The wheel
   * missed you." An older server leaves the key out, so read it with `=== true`.
   */
  missed_spin: boolean;
  /**
   * Phase 3: the entry of the shown spin's date, or null (no entry, it was deleted, or an admin
   * removed it: `entry_removed`). A Phase 2 server leaves the key out, so read it with `!today.entry`.
   */
  entry: TodayEntry | null;
  /**
   * An admin removed the shown spin's post: `entry` is null, and the day still counts. Left out
   * otherwise, so read it with `=== true`.
   */
  entry_removed?: boolean;
  /** The spin's own wheel when a spin is shown, else the live wheel. */
  wheel: WheelEntry[];
  week: Week;
  bonus: Bonus;
  /**
   * True only while the wheel is open because this is the person's first spin ever: they have never
   * spun and it is before 8:00 PM, so the wheel opens at once and that spin counts as today's. An
   * older server leaves the key out, so read it with `=== true`.
   */
  first_spin: boolean;
}

export interface SpinResponse {
  /** True when the server already had this spin (a retry, or another phone). */
  already: boolean;
  today: Today;
}

/**
 * The answer of POST /time-zone: the zone as stored, and Today as of the same moment, so the phone
 * needs no GET /today after it (2026-11-16). An older server leaves `today` out.
 */
export interface TimeZoneResponse {
  time_zone: string;
  today?: Today;
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

import { useEffect, useSyncExternalStore } from 'react';
import type { Today, WheelEntry } from '@/lib/api/types';
import { parseInstant } from '@/lib/wheel/countdown';

/**
 * Today's state, shared by the Today screen, the middle Spin button and the Edit wheel screen
 * (plan 02-02, binding decisions 4 to 7). A module store read with `useSyncExternalStore`.
 *
 * It is keyed by the signed-in user id. A read under another id sees nothing and discards the
 * old data (see `useTodaySnapshot`), any write under another id discards the old data first,
 * and an answer whose request began under another id is dropped, so a second account never
 * sees the first one's wheel, result or pending spin. `lastReportedZone` lives here too and,
 * like everything else, is in memory only.
 */

export type LoadFailure = 'network' | 'generic';
export type SpinFailureKind = 'network' | 'not_open' | 'closed' | 'generic';

/** A spin the screen has been given to animate. The wheel turns when `id` rises. */
export type Turn = {
  id: number;
  winnerIndex: number | null;
  /** The wheel the turn is drawn on (the spin's own wheel). */
  entries: WheelEntry[] | null;
};

/**
 * What the countdown counts from: the server's `now` (milliseconds) and the monotonic mark
 * (`performance.now()`) taken when the request that brought it started. Never the phone's clock.
 */
export type Anchor = { serverMs: number; mark: number };

export type TodaySnapshot = {
  userId: string | null;
  today: Today | null;
  anchor: Anchor | null;
  /** The last load failed and nothing is on screen. */
  loadFailure: LoadFailure | null;
  /** A spin request is in flight. */
  requesting: 'spin' | 'bonus' | null;
  /** The last spin or bonus request failed. `action` says which one a retry sends again. */
  spinFailure: { kind: SpinFailureKind; action: 'spin' | 'bonus' } | null;
  /** The wheel is turning (from the answer until it lands). Refetch results are held. */
  spinning: boolean;
  /** The middle Spin button was pressed and Today has not acted on it yet. */
  pendingSpin: boolean;
  lastReportedZone: string | null;
  turn: Turn;
  /** A trigger was ignored while the wheel turned: one refetch runs when it lands. */
  refetchOnLanding: boolean;
};

const EMPTY_TURN: Turn = { id: 0, winnerIndex: null, entries: null };

function emptySnapshot(userId: string | null): TodaySnapshot {
  return {
    userId,
    today: null,
    anchor: null,
    loadFailure: null,
    requesting: null,
    spinFailure: null,
    spinning: false,
    pendingSpin: false,
    lastReportedZone: null,
    turn: EMPTY_TURN,
    refetchOnLanding: false,
  };
}

const EMPTY: TodaySnapshot = emptySnapshot(null);

let state: TodaySnapshot = EMPTY;
const listeners = new Set<() => void>();

function emit(): void {
  for (const listener of Array.from(listeners)) listener();
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

function getSnapshot(): TodaySnapshot {
  return state;
}

/** What `userId` may see: its own data, or nothing. Never mutates. */
export function readToday(userId: string | null): TodaySnapshot {
  return state.userId === userId && userId !== null ? state : EMPTY;
}

/** Takes the store for `userId`, discarding another account's data. */
export function claimToday(userId: string | null): void {
  if (state.userId === userId) return;
  state = emptySnapshot(userId);
  emit();
}

/** A write by the person who is on screen: claims the store first, then merges `patch`. */
export function patchToday(userId: string, patch: Partial<Omit<TodaySnapshot, 'userId'>>): void {
  if (state.userId !== userId) state = emptySnapshot(userId);
  state = { ...state, ...patch };
  emit();
}

/**
 * A write made when an answer arrives: applied only while the store still belongs to the
 * account whose request it was, otherwise dropped. Returns whether it was applied.
 */
export function patchIfOwner(userId: string, patch: Partial<Omit<TodaySnapshot, 'userId'>>): boolean {
  if (state.userId !== userId) return false;
  state = { ...state, ...patch };
  emit();
  return true;
}

/**
 * Puts an answer that began under `userId` into the store. Returns false (and writes nothing)
 * when the store already belongs to another account, so a late answer from a signed-out
 * session can never reach the next person. Phase 3 calls `applyToday` with the Today inside a
 * post or skip answer through the same rule.
 *
 * `mark` is `performance.now()` taken when the request started; the countdown counts from it.
 */
export function applyToday(
  userId: string,
  today: Today,
  mark: number,
  extra: Partial<Omit<TodaySnapshot, 'userId' | 'today' | 'anchor'>> = {}
): boolean {
  if (state.userId !== null && state.userId !== userId) return false;
  const serverMs = parseInstant(today.now);
  state = {
    ...(state.userId === userId ? state : emptySnapshot(userId)),
    today,
    anchor: serverMs === null ? null : { serverMs, mark },
    loadFailure: null,
    ...extra,
  };
  emit();
  return true;
}

/** Drops the countdown anchor (on foreground): the digits show their placeholder until a fresh answer. */
export function dropAnchor(userId: string): void {
  if (state.userId !== userId || state.anchor === null) return;
  state = { ...state, anchor: null };
  emit();
}

/** The middle Spin button: ask Today to spin once it has loaded and is still open. */
export function requestPendingSpin(userId: string): void {
  patchToday(userId, { pendingSpin: true });
}

/** Today (or the leave sheet) has dealt with the request, or it must not happen. */
export function clearPendingSpin(userId: string | null): void {
  if (userId === null || state.userId !== userId || !state.pendingSpin) return;
  state = { ...state, pendingSpin: false };
  emit();
}

/**
 * Reads the store for `userId` inside a component. The first read under a new id discards the
 * previous account's data.
 */
export function useTodaySnapshot(userId: string | null): TodaySnapshot {
  const snapshot = useSyncExternalStore(subscribe, getSnapshot, getSnapshot);
  useEffect(() => {
    claimToday(userId);
  }, [userId]);
  return snapshot.userId === userId && userId !== null ? snapshot : EMPTY;
}

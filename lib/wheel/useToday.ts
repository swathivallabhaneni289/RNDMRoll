import { useCallback, useEffect, useRef } from 'react';
import { AppState } from 'react-native';
import { useFocusEffect, useIsFocused } from 'expo-router';
import { ApiError } from '@/lib/api/client';
import { getToday, reportTimeZone, spin as spinRequest, spinBonus as spinBonusRequest } from '@/lib/api/spin';
import { msUntil, nextWaitMs } from '@/lib/wheel/countdown';
import { deviceTimeZone } from '@/lib/wheel/timeZone';
import {
  applyToday,
  clearPendingSpin,
  dropAnchor,
  patchIfOwner,
  patchToday,
  readToday,
  useTodaySnapshot,
  type SpinFailureKind,
} from '@/lib/wheel/todayStore';
import { useSession } from '@/lib/session/store';

/**
 * Loads and spins Today (plan 02-02, binding decisions 4 to 7). The phone shows what the server
 * says: it never works out the date, the window, the winner or a time.
 *
 * Why a module and not only a hook: the rules below ("never two requests at once", "one refetch
 * at a time") must hold even if two components ask, so the queue lives here and the hook only
 * wires the triggers.
 *
 *  - One request runs at a time. A load asked for while another request runs waits behind it,
 *    and several waiting loads become one.
 *  - Today is asked for on mount, on tab focus, on foreground, when a countdown reaches zero
 *    and at the server's `next_open_at` and close time (nearest first, re-armed after every
 *    answer). If the answer at zero still says waiting, ask again after 1 second, at most 5
 *    times.
 *  - While the wheel turns, triggers are ignored and one refetch runs when it lands.
 *  - After `spin_closed` or `spin_not_open` Today is asked for again and the message shows
 *    once; "Try again" for those is a refetch, not a new POST. Only a network failure sends
 *    the same request again.
 *  - A failed refetch with data on screen keeps the screen and tries once more after 5 seconds.
 *  - The time zone is reported on mount, on foreground and when it differs from the last one
 *    reported, never on tab focus. A failed report does not block Today.
 */

export type LoadTrigger =
  | 'mount'
  | 'focus'
  | 'foreground'
  | 'timer'
  | 'zero-retry'
  | 'retry'
  | 'landed'
  | 'pending'
  | 'followup'
  | 'manual';

const ZERO_RETRY_MS = 1000;
const ZERO_RETRIES = 5;
const FAILED_REFETCH_RETRY_MS = 5000;
/** setTimeout's largest delay. */
const MAX_TIMER_MS = 2 ** 31 - 1;

// ---------------------------------------------------------------------------
// The request queue.
// ---------------------------------------------------------------------------

let tail: Promise<unknown> = Promise.resolve();
let queuedLoad: Promise<void> | null = null;
let queuedLoadUser: string | null = null;

function enqueue<T>(job: () => Promise<T>): Promise<T> {
  const run = tail.then(job, job);
  tail = run.then(
    () => undefined,
    () => undefined
  );
  return run;
}

let zeroAttempts = 0;
let zeroTimer: ReturnType<typeof setTimeout> | null = null;
let failedRefetchTimer: ReturnType<typeof setTimeout> | null = null;
let failedRefetchUsed = false;
let lastFiredMark: number | null = null;

function failureKindOf(err: unknown): 'network' | 'generic' {
  return err instanceof ApiError && err.code === 'network_unavailable' ? 'network' : 'generic';
}

function spinFailureKindOf(err: unknown): SpinFailureKind {
  if (err instanceof ApiError) {
    if (err.code === 'network_unavailable') return 'network';
    if (err.code === 'spin_not_open') return 'not_open';
    if (err.code === 'spin_closed') return 'closed';
  }
  return 'generic';
}

async function loadNow(userId: string, trigger: LoadTrigger): Promise<void> {
  const before = readToday(userId);
  if (before.spinning) {
    // The wheel is turning: ignore the trigger, one refetch runs when it lands.
    patchIfOwner(userId, { refetchOnLanding: true });
    return;
  }

  const zone = deviceTimeZone();
  if (zone === null) {
    // The plain "Something went wrong" message. Nothing is stored.
    if (!before.today) patchToday(userId, { loadFailure: 'generic' });
    return;
  }

  // Reported on mount, on foreground and on a change, never on tab focus.
  const mustReport =
    trigger === 'mount' || trigger === 'foreground' || (trigger !== 'focus' && zone !== before.lastReportedZone);
  if (mustReport) {
    try {
      await reportTimeZone(zone);
      patchIfOwner(userId, { lastReportedZone: zone });
    } catch {
      // A failed report never blocks Today (the server uses the stored zone). It is tried
      // again at the next trigger, because lastReportedZone did not change.
    }
  }

  const mark = performance.now();
  try {
    const today = await getToday();
    const keep = trigger === 'followup' ? {} : { spinFailure: null };
    if (applyToday(userId, today, mark, keep)) {
      failedRefetchUsed = false;
      if (failedRefetchTimer) {
        clearTimeout(failedRefetchTimer);
        failedRefetchTimer = null;
      }
    }
  } catch (err) {
    const current = readToday(userId);
    if (current.today) {
      // Data on screen: keep the screen and try once more after 5 seconds.
      if (!failedRefetchUsed && failedRefetchTimer === null) {
        failedRefetchTimer = setTimeout(() => {
          failedRefetchTimer = null;
          failedRefetchUsed = true;
          void refetchToday(userId, 'retry');
        }, FAILED_REFETCH_RETRY_MS);
      }
    } else {
      patchIfOwner(userId, { loadFailure: failureKindOf(err) });
    }
  }
}

/**
 * Asks the server for Today. The promise settles when a load that started after this call has
 * finished (so a caller can act on a fresh answer). Calls made together become one request.
 */
export function refetchToday(userId: string, trigger: LoadTrigger): Promise<void> {
  if (readToday(userId).spinning) {
    patchIfOwner(userId, { refetchOnLanding: true });
    return Promise.resolve();
  }
  if (queuedLoad && queuedLoadUser === userId) return queuedLoad;
  const job: Promise<void> = enqueue(async () => {
    if (queuedLoad === job) {
      queuedLoad = null;
      queuedLoadUser = null;
    }
    await loadNow(userId, trigger);
  });
  queuedLoad = job;
  queuedLoadUser = userId;
  return job;
}

/** After a load made at a server instant: if it still says waiting with nothing ahead, ask again soon. */
function afterInstantLoad(userId: string): void {
  const snap = readToday(userId);
  const today = snap.today;
  if (today && today.state === 'waiting' && snap.anchor) {
    const left = msUntil(today.next_open_at, snap.anchor, performance.now());
    if ((left === null || left <= 0) && zeroAttempts < ZERO_RETRIES) {
      zeroAttempts += 1;
      if (zeroTimer) clearTimeout(zeroTimer);
      zeroTimer = setTimeout(() => {
        zeroTimer = null;
        void refetchToday(userId, 'zero-retry').then(() => afterInstantLoad(userId));
      }, ZERO_RETRY_MS);
      return;
    }
  }
  zeroAttempts = 0;
}

/** A countdown reached zero, or the server's next-open or close time came. Once per answer. */
export function instantReached(userId: string, mark: number): void {
  if (lastFiredMark === mark) return;
  lastFiredMark = mark;
  void refetchToday(userId, 'timer').then(() => afterInstantLoad(userId));
}

// ---------------------------------------------------------------------------
// Spinning.
// ---------------------------------------------------------------------------

async function runSpin(userId: string, action: 'spin' | 'bonus', afterSeq: number): Promise<void> {
  const before = readToday(userId);
  if (!before.today || before.spinning || before.requesting) return;
  const forDate = before.today.local_date;
  patchToday(userId, { requesting: action, spinFailure: null });

  await enqueue(async () => {
    const mark = performance.now();
    try {
      const response =
        action === 'spin' ? await spinRequest(forDate) : await spinBonusRequest(afterSeq, forDate);
      const spun = response.today.spin;
      if (!spun) throw new ApiError(200, 'server_error');
      const current = readToday(userId);
      // The turn is handed to the screen together with the answer, in one write.
      applyToday(userId, response.today, mark, {
        requesting: null,
        spinFailure: null,
        spinning: true,
        turn: { id: current.turn.id + 1, winnerIndex: spun.winner_index, entries: spun.wheel },
      });
    } catch (err) {
      const kind = spinFailureKindOf(err);
      patchIfOwner(userId, { requesting: null, spinFailure: { kind, action } });
      if (kind === 'not_open' || kind === 'closed') {
        // Show the message once and ask the server again. Not awaited: it waits behind this job.
        void refetchToday(userId, 'followup');
      }
    }
  });
}

/** The day's daily spin. */
export function spinToday(userId: string): Promise<void> {
  return runSpin(userId, 'spin', 0);
}

/** A bonus spin, with the `seq` of the spin that was on screen when the person said yes. */
export function spinBonusToday(userId: string, afterSeq: number): Promise<void> {
  return runSpin(userId, 'bonus', afterSeq);
}

/**
 * The wheel has landed: it no longer turns, so held answers apply. One refetch runs if a
 * trigger was ignored while it turned.
 */
export function wheelLanded(userId: string): void {
  const snap = readToday(userId);
  if (!snap.spinning) return;
  patchIfOwner(userId, { spinning: false, refetchOnLanding: false });
  if (snap.refetchOnLanding) void refetchToday(userId, 'landed');
}

/**
 * "Try again" under a message. A lost connection sends the same request again (the server
 * returns the same spin for a retry); every other failure asks for Today instead.
 */
export function tryAgainAfterFailure(userId: string, bonusAfterSeq: number): void {
  const snap = readToday(userId);
  const failure = snap.spinFailure;
  if (!failure) {
    void refetchToday(userId, 'manual');
    return;
  }
  patchIfOwner(userId, { spinFailure: null });
  if (failure.kind === 'network') {
    if (failure.action === 'spin') void spinToday(userId);
    else void spinBonusToday(userId, bonusAfterSeq);
  } else {
    void refetchToday(userId, 'manual');
  }
}

// ---------------------------------------------------------------------------
// The hook.
// ---------------------------------------------------------------------------

/**
 * Wires Today's triggers (focus, foreground, the server's instants) and returns the store's
 * snapshot for the signed-in person. Mount it once, in the Today screen.
 */
export function useToday() {
  const { user } = useSession();
  const userId = user?.id ?? null;
  const snapshot = useTodaySnapshot(userId);
  const isFocused = useIsFocused();
  const firstFocusRef = useRef(true);

  /** Acts on the middle Spin button once the answer is fresh: spin if still open, else just clear it. */
  const consumePendingSpin = useCallback(() => {
    if (userId === null) return;
    const snap = readToday(userId);
    if (!snap.pendingSpin) return;
    clearPendingSpin(userId);
    if (snap.today?.state === 'open' && !snap.spinning && !snap.requesting) {
      void spinToday(userId);
    }
  }, [userId]);

  // Mount (the first focus) and every later tab focus.
  useFocusEffect(
    useCallback(() => {
      if (userId === null) return undefined;
      const trigger: LoadTrigger = firstFocusRef.current ? 'mount' : 'focus';
      firstFocusRef.current = false;
      void refetchToday(userId, trigger).then(consumePendingSpin);
      return undefined;
    }, [userId, consumePendingSpin])
  );

  // The middle Spin button pressed while Today is already on screen.
  useEffect(() => {
    if (userId === null || !snapshot.pendingSpin || !isFocused) return;
    void refetchToday(userId, 'pending').then(consumePendingSpin);
  }, [userId, snapshot.pendingSpin, isFocused, consumePendingSpin]);

  // Foreground: drop the anchor (the digits show a placeholder) and ask again.
  useEffect(() => {
    if (userId === null) return undefined;
    let wasBackgrounded = false;
    const subscription = AppState.addEventListener('change', (next) => {
      if (next === 'background') wasBackgrounded = true;
      if (next === 'active' && wasBackgrounded) {
        wasBackgrounded = false;
        dropAnchor(userId);
        void refetchToday(userId, 'foreground');
      }
    });
    return () => subscription.remove();
  }, [userId]);

  // The server's next opening and close time, nearest first, re-armed after every answer.
  const today = snapshot.today;
  const anchor = snapshot.anchor;
  useEffect(() => {
    if (userId === null || !today || !anchor || snapshot.spinning) return undefined;
    const wait = nextWaitMs([today.next_open_at, today.window.closes_at], anchor, performance.now());
    if (wait === null) return undefined;
    const id = setTimeout(() => instantReached(userId, anchor.mark), Math.min(wait, MAX_TIMER_MS));
    return () => clearTimeout(id);
  }, [userId, today, anchor, snapshot.spinning]);

  // Nothing carries over to the next screen visit except what is in the store.
  useEffect(
    () => () => {
      if (zeroTimer) {
        clearTimeout(zeroTimer);
        zeroTimer = null;
      }
      if (failedRefetchTimer) {
        clearTimeout(failedRefetchTimer);
        failedRefetchTimer = null;
      }
    },
    []
  );

  return {
    userId,
    snapshot,
    refetch: (trigger: LoadTrigger = 'manual') =>
      userId === null ? Promise.resolve() : refetchToday(userId, trigger),
    spin: () => (userId === null ? Promise.resolve() : spinToday(userId)),
    bonus: (afterSeq: number) => (userId === null ? Promise.resolve() : spinBonusToday(userId, afterSeq)),
    landed: () => {
      if (userId !== null) wheelLanded(userId);
    },
    tryAgain: (bonusAfterSeq: number) => {
      if (userId !== null) tryAgainAfterFailure(userId, bonusAfterSeq);
    },
    instantReached: () => {
      if (userId !== null && anchor) instantReached(userId, anchor.mark);
    },
  };
}

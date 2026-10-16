import { useCallback, useEffect, useRef, useState } from 'react';
import { useFocusEffect } from 'expo-router';
import { ApiError } from '@/lib/api/client';
import { getBoard, type Board, type BoardDay } from '@/lib/api/board';
import { useSession } from '@/lib/session/store';

/** Five days a page (05-UI-SPEC 3.5); the server's default too. */
const PAGE_DAYS = 5;

type Held = { ownerId: string | null; board: Board | null };

export type BoardLoad = {
  /** `loading` until the first answer; `offline` only when nothing is on screen. */
  status: 'loading' | 'ready' | 'offline';
  failure: 'network' | 'generic';
  board: Board | null;
  refreshing: boolean;
  loadingMore: boolean;
  moreFailed: boolean;
  retry: () => void;
  refresh: () => void;
  loadMore: () => void;
};

function failureKind(err: unknown): 'network' | 'generic' {
  return err instanceof ApiError && err.code === 'network_unavailable' ? 'network' : 'generic';
}

/** A later page appended: a day already shown gets the new cards (by entry id), new days follow. */
function mergeDays(current: BoardDay[], next: BoardDay[]): BoardDay[] {
  const out = current.map((day) => ({ ...day, cards: [...day.cards] }));
  for (const day of next) {
    const same = out.find((shown) => shown.local_date === day.local_date);
    if (!same) {
      out.push(day);
      continue;
    }
    for (const card of day.cards) {
      if (!same.cards.some((shown) => shown.entry_id === card.entry_id)) same.cards.push(card);
    }
  }
  return out;
}

/**
 * Loads the Board (05-UI-SPEC 3.5). The first page loads when the tab comes into focus and again
 * each time it does, quietly: what is on screen stays while it runs and when it fails. The next
 * page is asked with the server's `next_before`, one at a time. Answers that arrive after a newer
 * first page began are dropped. What belongs to another account is never shown.
 */
export function useBoard(): BoardLoad {
  const { user } = useSession();
  const userId = user?.id ?? null;

  const [held, setHeld] = useState<Held>({ ownerId: userId, board: null });
  const [status, setStatus] = useState<BoardLoad['status']>('loading');
  const [failure, setFailure] = useState<BoardLoad['failure']>('network');
  const [refreshing, setRefreshing] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [moreFailed, setMoreFailed] = useState(false);

  const heldRef = useRef(held);
  const seqRef = useRef(0);
  const moreBusyRef = useRef(false);
  const mountedRef = useRef(true);

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);

  const put = useCallback((next: Held) => {
    heldRef.current = next;
    setHeld(next);
  }, []);

  const loadFirst = useCallback(
    async (mode: 'quiet' | 'pull' | 'retry') => {
      if (userId === null) return;
      const mine = ++seqRef.current;
      moreBusyRef.current = false;
      setLoadingMore(false);
      if (mode === 'pull') setRefreshing(true);
      if (mode === 'retry') setStatus('loading');
      try {
        const board = await getBoard({ limit: PAGE_DAYS });
        if (!mountedRef.current || mine !== seqRef.current) return;
        put({ ownerId: userId, board });
        setMoreFailed(false);
        setStatus('ready');
      } catch (err) {
        if (!mountedRef.current || mine !== seqRef.current) return;
        const now = heldRef.current;
        if (now.board !== null && now.ownerId === userId) return;
        setFailure(failureKind(err));
        setStatus('offline');
      } finally {
        if (mountedRef.current && mode === 'pull') setRefreshing(false);
      }
    },
    [userId, put]
  );

  const loadMore = useCallback(async () => {
    const shown = heldRef.current;
    const before = shown.board?.next_before ?? null;
    if (userId === null || before === null || moreBusyRef.current || shown.ownerId !== userId) return;
    moreBusyRef.current = true;
    const mine = seqRef.current;
    setLoadingMore(true);
    setMoreFailed(false);
    try {
      const page = await getBoard({ before, limit: PAGE_DAYS });
      if (!mountedRef.current || mine !== seqRef.current) return;
      const now = heldRef.current.board;
      if (now === null) return;
      put({
        ownerId: userId,
        board: { ...now, days: mergeDays(now.days, page.days), next_before: page.next_before },
      });
    } catch {
      if (mountedRef.current && mine === seqRef.current) setMoreFailed(true);
    } finally {
      if (mine === seqRef.current) {
        moreBusyRef.current = false;
        if (mountedRef.current) setLoadingMore(false);
      }
    }
  }, [userId, put]);

  // A change of account clears what is shown at once.
  useEffect(() => {
    if (heldRef.current.ownerId !== userId) {
      put({ ownerId: userId, board: null });
      setStatus('loading');
    }
  }, [userId, put]);

  useFocusEffect(
    useCallback(() => {
      void loadFirst('quiet');
    }, [loadFirst])
  );

  const board = held.ownerId === userId ? held.board : null;

  return {
    status: board === null && status === 'ready' ? 'loading' : status,
    failure,
    board,
    refreshing,
    loadingMore,
    moreFailed,
    retry: () => void loadFirst('retry'),
    refresh: () => void loadFirst('pull'),
    loadMore: () => void loadMore(),
  };
}

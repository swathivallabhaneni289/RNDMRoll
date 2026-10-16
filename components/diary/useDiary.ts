import { useCallback, useEffect, useRef, useState } from 'react';
import { useFocusEffect } from 'expo-router';
import { ApiError } from '@/lib/api/client';
import { getDiary, type DiaryCategory, type DiaryItem } from '@/lib/api/diary';
import { useSession } from '@/lib/session/store';

/** 20 rows a page (04-UI-SPEC 3.6); the server's own default too. */
const PAGE_SIZE = 20;
/** The most the server sends in one page: a quiet reload asks for up to this many to keep the place. */
const MAX_PAGE = 50;

/** A row's key: the entry id, or the skipped day's date (one row per date). */
export function rowKey(item: DiaryItem): string {
  return item.kind === 'entry' ? item.entry.id : `skipped-${item.local_date}`;
}

type Held = {
  ownerId: string | null;
  /** The category the rows were loaded for (null is All). */
  categoryId: string | null;
  rows: DiaryItem[];
  chips: DiaryCategory[];
  nextCursor: string | null;
  /** A first page has arrived for this person. */
  loaded: boolean;
};

export type DiaryState = {
  /** `loading` until the first page or failure; `offline` only when nothing is on screen. */
  status: 'loading' | 'ready' | 'offline';
  failure: 'network' | 'generic';
  rows: DiaryItem[];
  chips: DiaryCategory[];
  /** The category the rows on screen belong to (they stay while a new chip loads). */
  rowsFor: string | null;
  refreshing: boolean;
  loadingMore: boolean;
  moreFailed: boolean;
  /** 'Try again' after a failed first load. */
  retry: () => void;
  /** Pull to refresh. */
  refresh: () => void;
  /** Reads the list again without the pull spinner (after an action from a row's menu). */
  reload: () => void;
  /** The end of the list came into view: the next page, once at a time. */
  loadMore: () => void;
  /** 'Try again' under the list after a failed next page. */
  retryMore: () => void;
};

function failureKind(err: unknown): 'network' | 'generic' {
  return err instanceof ApiError && err.code === 'network_unavailable' ? 'network' : 'generic';
}

/**
 * Loads the Diary for one category (null is All), newest first (04-UI-SPEC 3.5 and 3.6).
 *
 *  - The first page loads when the tab comes into focus and again each time it does, and when
 *    the chip changes; rows already shown stay while it runs, and a failed reload keeps them.
 *    A reload on focus asks for as many rows as are shown (up to 50), so coming Back from an
 *    entry keeps the person's place in a long list.
 *  - The next page is asked with the server's cursor, one request at a time; a failure keeps
 *    the rows and waits for 'Try again'. Rows already shown are never doubled.
 *  - Answers that arrive after a newer request began are dropped (a chip change, a pull).
 *  - What belongs to another account is never shown.
 */
export function useDiary(categoryId: string | null): DiaryState {
  const { user } = useSession();
  const userId = user?.id ?? null;

  const [held, setHeld] = useState<Held>({
    ownerId: userId,
    categoryId,
    rows: [],
    chips: [],
    nextCursor: null,
    loaded: false,
  });
  const [status, setStatus] = useState<DiaryState['status']>('loading');
  const [failure, setFailure] = useState<DiaryState['failure']>('network');
  const [refreshing, setRefreshing] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [moreFailed, setMoreFailed] = useState(false);

  const heldRef = useRef(held);
  const seqRef = useRef(0);
  const moreBusyRef = useRef(false);
  const firstBusyRef = useRef(false);
  const moreFailedRef = useRef(false);
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

  const setMoreFailedBoth = useCallback((value: boolean) => {
    moreFailedRef.current = value;
    setMoreFailed(value);
  }, []);

  const loadFirst = useCallback(
    async (mode: 'quiet' | 'pull' | 'retry') => {
      if (userId === null) return;
      const mine = ++seqRef.current;
      // A newer first page replaces any next page still on its way, and none starts until it ends.
      firstBusyRef.current = true;
      moreBusyRef.current = false;
      setLoadingMore(false);
      if (mode === 'pull') setRefreshing(true);
      if (mode === 'retry') setStatus('loading');
      const shown = heldRef.current;
      const keepPlace = mode === 'quiet' && shown.loaded && shown.ownerId === userId && shown.categoryId === categoryId;
      const limit = keepPlace ? Math.min(MAX_PAGE, Math.max(PAGE_SIZE, shown.rows.length)) : PAGE_SIZE;
      try {
        const page = await getDiary({ categoryId, limit });
        if (!mountedRef.current || mine !== seqRef.current) return;
        put({
          ownerId: userId,
          categoryId,
          rows: page.items,
          chips: page.categories,
          nextCursor: page.next_cursor,
          loaded: true,
        });
        setMoreFailedBoth(false);
        setStatus('ready');
      } catch (err) {
        if (!mountedRef.current || mine !== seqRef.current) return;
        // Something of this person is on screen: keep it and show nothing new.
        const now = heldRef.current;
        if (now.loaded && now.ownerId === userId) return;
        setFailure(failureKind(err));
        setStatus('offline');
      } finally {
        if (mine === seqRef.current) firstBusyRef.current = false;
        if (mountedRef.current && mode === 'pull') setRefreshing(false);
      }
    },
    [userId, categoryId, put, setMoreFailedBoth]
  );

  const fetchMore = useCallback(
    async (auto: boolean) => {
      const now = heldRef.current;
      if (userId === null || moreBusyRef.current || firstBusyRef.current) return;
      if (!now.loaded || now.ownerId !== userId || now.categoryId !== categoryId || now.nextCursor === null) return;
      // After a failure the list waits for 'Try again' instead of asking on every scroll.
      if (auto && moreFailedRef.current) return;
      moreBusyRef.current = true;
      const mine = seqRef.current;
      setLoadingMore(true);
      setMoreFailedBoth(false);
      try {
        const page = await getDiary({ categoryId, cursor: now.nextCursor, limit: PAGE_SIZE });
        if (!mountedRef.current || mine !== seqRef.current) return;
        const current = heldRef.current;
        const seen = new Set(current.rows.map(rowKey));
        put({
          ...current,
          rows: [...current.rows, ...page.items.filter((item) => !seen.has(rowKey(item)))],
          chips: page.categories,
          nextCursor: page.next_cursor,
        });
      } catch {
        if (!mountedRef.current || mine !== seqRef.current) return;
        setMoreFailedBoth(true);
      } finally {
        if (mine === seqRef.current) {
          moreBusyRef.current = false;
          if (mountedRef.current) setLoadingMore(false);
        }
      }
    },
    [userId, categoryId, put, setMoreFailedBoth]
  );

  // The first page on every focus and on every chip change while focused.
  useFocusEffect(
    useCallback(() => {
      void loadFirst('quiet');
    }, [loadFirst])
  );

  const mine = held.ownerId === userId;
  return {
    status: mine ? status : 'loading',
    failure,
    rows: mine ? held.rows : [],
    chips: mine ? held.chips : [],
    rowsFor: held.categoryId,
    refreshing,
    loadingMore,
    moreFailed,
    retry: () => void loadFirst('retry'),
    refresh: () => void loadFirst('pull'),
    reload: () => void loadFirst('quiet'),
    loadMore: () => void fetchMore(true),
    retryMore: () => void fetchMore(false),
  };
}

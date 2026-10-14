import { useCallback, useEffect, useRef, useState } from 'react';
import { useFocusEffect } from 'expo-router';
import { ApiError } from '@/lib/api/client';
import { getEntry, type Entry } from '@/lib/api/entries';
import { isEntryId } from '@/lib/entry/rules';

export type EntryLoadStatus = 'loading' | 'ready' | 'offline' | 'gone';

export type EntryLoad = {
  status: EntryLoadStatus;
  /** The entry as last read. Kept on screen while a later read runs or fails. */
  entry: Entry | null;
  /** For `offline`: the lost connection (`network`) or any other failure (`generic`). */
  failure: 'network' | 'generic';
  /** Reads again ('Try again'). */
  reload: () => void;
};

/**
 * Loads one entry by id (plan 03-03 Task 1b). Reports `loading`, `ready`, `offline` or `gone`, and
 * reads again whenever the screen comes back into focus, so an edit shows at once. While a later
 * read runs, or fails, the entry already on screen stays: only the first read shows 'loading', and
 * only a read with nothing on screen yet shows 'offline'.
 *
 * An id that is not a UUID is `gone` at once and is never sent to the server.
 */
export function useEntry(id: string): EntryLoad {
  const valid = isEntryId(id);
  const [entry, setEntry] = useState<Entry | null>(null);
  const [status, setStatus] = useState<EntryLoadStatus>(valid ? 'loading' : 'gone');
  const [failure, setFailure] = useState<'network' | 'generic'>('network');

  const entryRef = useRef<Entry | null>(null);
  const seqRef = useRef(0);
  const mountedRef = useRef(true);

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);

  const load = useCallback(async () => {
    if (!valid) {
      entryRef.current = null;
      setEntry(null);
      setStatus('gone');
      return;
    }
    // A newer read replaces an older one: only the last answer is used.
    const seq = ++seqRef.current;
    if (entryRef.current === null) setStatus('loading');
    try {
      const next = await getEntry(id);
      if (!mountedRef.current || seq !== seqRef.current) return;
      entryRef.current = next;
      setEntry(next);
      setStatus('ready');
    } catch (err) {
      if (!mountedRef.current || seq !== seqRef.current) return;
      if (err instanceof ApiError && err.code === 'not_found') {
        entryRef.current = null;
        setEntry(null);
        setStatus('gone');
        return;
      }
      // Something is on screen already: keep it. Otherwise say what went wrong.
      if (entryRef.current !== null) return;
      setFailure(err instanceof ApiError && err.code === 'network_unavailable' ? 'network' : 'generic');
      setStatus('offline');
    }
  }, [id, valid]);

  useFocusEffect(
    useCallback(() => {
      void load();
    }, [load])
  );

  return { status, entry, failure, reload: () => void load() };
}

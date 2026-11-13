import { useCallback, useEffect, useRef } from 'react';

/** A photo that will not load reads the screen again at most this often. */
const RELOAD_GAP_MS = 60_000;

/**
 * One quiet reload when a photo will not load (private photos, 2026-11-09). Its link may have run
 * out (a link stops working within an hour), and the screen's `reload` brings fresh ones. Only
 * once `stale` says the screen's first page is over 20 minutes old (useForegroundReload): younger
 * data holds no link that has run out, so a photo in it that fails (no connection for a moment, a
 * file that is gone) is not fixed by a reload, and the Board would only lose its later pages. A
 * reload makes the data young again, so a file that is gone reads the screen at most once in 20
 * minutes. And at most once a minute: with no connection the reload fails and the data stays old,
 * so failing photos never set off a loop. Returns one function for the life of the screen, for its
 * photos' `onError`; the newest `reload` and `stale` are the ones called.
 */
export function usePhotoReload(reload: () => void, stale: () => boolean): () => void {
  const reloadRef = useRef(reload);
  const staleRef = useRef(stale);
  const lastRef = useRef(-Infinity);

  useEffect(() => {
    reloadRef.current = reload;
    staleRef.current = stale;
  }, [reload, stale]);

  return useCallback(() => {
    if (!staleRef.current()) return;
    const now = Date.now();
    if (now - lastRef.current < RELOAD_GAP_MS) return;
    lastRef.current = now;
    reloadRef.current();
  }, []);
}

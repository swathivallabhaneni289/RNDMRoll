import { useCallback, useEffect, useMemo, useRef } from 'react';
import { AppState } from 'react-native';
import { useIsFocused } from 'expo-router';
import { isStale } from '@/lib/api/photoSource';

/**
 * Reads a screen's data again when the app comes back from the background and the last answer is
 * over 20 minutes old (STALE_AFTER_MS): the photo links in it stop working within an hour (private
 * photos, 2026-11-09). Only while the screen is in focus: the tabs and stacks keep screens mounted,
 * and one out of focus reads again on its next focus anyway (Search, with no reload on focus of its
 * own, checks `stale` when it comes back into focus). The return from the background is Today's
 * (lib/wheel/useToday.ts): a system sheet (a permission prompt, the photo picker) only makes the
 * app inactive, so it reads nothing.
 *
 * Returns `markLoaded`, which the screen calls when a read of its first page has answered: never
 * for a later page or an added comment, as the oldest link on screen is the first to run out.
 * Nothing is read again before the first answer. And `stale`, true once that answer is over 20
 * minutes old (a photo that will not load reads the screen again only then, usePhotoReload). The
 * newest `reload` is the one called.
 */
export function useForegroundReload(reload: () => void): { markLoaded: () => void; stale: () => boolean } {
  const isFocused = useIsFocused();
  const focusedRef = useRef(isFocused);
  const reloadRef = useRef(reload);
  // When the last read answered. Date.now(), not performance.now(): the time asleep must count.
  const loadedAtRef = useRef<number | null>(null);

  useEffect(() => {
    focusedRef.current = isFocused;
    reloadRef.current = reload;
  }, [isFocused, reload]);

  useEffect(() => {
    let wasBackgrounded = false;
    const subscription = AppState.addEventListener('change', (next) => {
      if (next === 'background') wasBackgrounded = true;
      if (next === 'active' && wasBackgrounded) {
        wasBackgrounded = false;
        if (focusedRef.current && isStale(loadedAtRef.current, Date.now())) reloadRef.current();
      }
    });
    return () => subscription.remove();
  }, []);

  const markLoaded = useCallback(() => {
    loadedAtRef.current = Date.now();
  }, []);
  const stale = useCallback(() => isStale(loadedAtRef.current, Date.now()), []);
  return useMemo(() => ({ markLoaded, stale }), [markLoaded, stale]);
}

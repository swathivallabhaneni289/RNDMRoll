import { createContext, useCallback, useContext, useEffect, useRef, useSyncExternalStore } from 'react';
import type { View } from 'react-native';

/**
 * Hand-rolled shared-element transition.
 *
 * Reanimated's built-in shared element transitions sit behind a static native
 * feature flag that is off in this build, so a list photo and the detail photo are
 * connected manually: the tapped photo's window rect is measured, the detail
 * screen animates a photo container from that rect to its own layout, and on
 * close it animates back to wherever the source photo is at that moment.
 *
 * State lives in a tiny module store rather than route params because a rect is
 * ephemeral and must never end up in a URL.
 */

export type Rect = { x: number; y: number; width: number; height: number };
export type HeroOrigin = Rect & { key: string; radius: number };

type Listener = () => void;

let origin: HeroOrigin | null = null;
let hiddenKey: string | null = null;
/** True from the tap that starts opening a detail until that detail is released. */
let opening = false;
const listeners = new Set<Listener>();
const sources = new Map<string, () => Promise<Rect | null>>();

/** A second tap before the first detail has mounted must not stack another one. */
const OPENING_FALLBACK_MS = 2500;

function emit() {
  listeners.forEach((listener) => listener());
}

function subscribe(listener: Listener) {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

export const hero = {
  getOrigin(): HeroOrigin | null {
    return origin;
  },
  setOrigin(next: HeroOrigin | null) {
    origin = next;
  },
  /** Claims the right to open a detail. False while another one is opening or open. */
  tryBegin(): boolean {
    if (opening) return false;
    opening = true;
    // If the detail never mounts (a failed navigation), do not lock the lists forever.
    setTimeout(() => {
      if (hiddenKey === null) opening = false;
    }, OPENING_FALLBACK_MS);
    return true;
  },
  /** Hides the source photo while its twin is in flight in the detail screen. */
  hide(key: string) {
    if (hiddenKey !== key) {
      hiddenKey = key;
      emit();
    }
  },
  /** Shows the source photo again and forgets the origin. */
  release() {
    origin = null;
    opening = false;
    if (hiddenKey !== null) {
      hiddenKey = null;
      emit();
    }
  },
  /** Where the source photo is right now, or null if it is unmounted, off screen or clipped. */
  async measureSource(key: string): Promise<Rect | null> {
    const measure = sources.get(key);
    return measure ? measure() : null;
  },
};

/**
 * Top edge, in window coordinates, below which a source list is actually visible
 * (the status bar cover, or the diary header). measureInWindow reports a view's full
 * frame even when part of it is scrolled under such a cover, so a photo clipped past
 * this edge cannot be flown honestly and falls back to fading up in place.
 */
export const HeroClipContext = createContext(0);
const CLIP_TOLERANCE = 2;

/**
 * Wire a list photo to the transition. `key` must be unique per surface, not just
 * per entry: the feed and the diary can be mounted at the same time and can show
 * the same entry.
 *
 * Attach `ref` to the photo container (with collapsable={false} so Android keeps
 * a native view to measure), call `open()` from onPress and navigate only if it
 * resolves true, and fade the container out while `hidden` is true.
 */
export function useHeroSource(key: string, radius: number) {
  const ref = useRef<View>(null);
  const clipTop = useContext(HeroClipContext);

  const measure = useCallback(
    () =>
      new Promise<Rect | null>((resolve) => {
        const node = ref.current;
        if (!node) {
          resolve(null);
          return;
        }
        node.measureInWindow((x, y, width, height) => {
          const clipped = clipTop > 0 && y < clipTop - CLIP_TOLERANCE;
          resolve(width > 0 && height > 0 && !clipped ? { x, y, width, height } : null);
        });
      }),
    [clipTop],
  );

  useEffect(() => {
    sources.set(key, measure);
    return () => {
      if (sources.get(key) === measure) sources.delete(key);
    };
  }, [key, measure]);

  /**
   * Resolves true when the caller should navigate. When the photo cannot be flown
   * (unmounted, or clipped by the list edge) the detail still opens, with no origin,
   * and fades up in place. Resolves false only for a repeat tap.
   */
  const open = useCallback(async () => {
    if (!hero.tryBegin()) return false;
    const rect = await measure();
    hero.setOrigin(rect ? { key, radius, ...rect } : null);
    return true;
  }, [key, measure, radius]);

  const hidden = useSyncExternalStore(
    subscribe,
    () => hiddenKey === key,
    () => false,
  );

  return { ref, open, hidden };
}

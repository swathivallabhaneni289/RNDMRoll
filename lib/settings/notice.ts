import { useSyncExternalStore } from 'react';

/**
 * The one-line notice Settings shows after the Username or Password page returns to it (plan
 * 02.1-02, decision 6). A page sets the text just before going back, Settings reads it and the
 * store clears itself after 3 seconds; a new text restarts the timer. It is a small module store
 * rather than a route parameter, so no text ever travels in a route (a password never could: the
 * only texts are the two fixed notices).
 */

const NOTICE_MS = 3000;

let current = '';
let timer: ReturnType<typeof setTimeout> | null = null;
const listeners = new Set<() => void>();

function emit(): void {
  listeners.forEach((listener) => listener());
}

/** Shows `text` for 3 seconds. An empty text clears the notice at once. */
export function setSettingsNotice(text: string): void {
  if (timer !== null) {
    clearTimeout(timer);
    timer = null;
  }
  current = text;
  if (text !== '') {
    timer = setTimeout(() => {
      timer = null;
      current = '';
      emit();
    }, NOTICE_MS);
  }
  emit();
}

/** The text now, '' when there is none. */
export function getSettingsNotice(): string {
  return current;
}

export function subscribeSettingsNotice(listener: () => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

/** The notice text for a screen: '' when there is none. */
export function useSettingsNotice(): string {
  return useSyncExternalStore(subscribeSettingsNotice, getSettingsNotice, getSettingsNotice);
}

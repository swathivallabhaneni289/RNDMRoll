import { useSyncExternalStore } from 'react';

/**
 * The one line after a report, "Report sent." (moderation, 2026-11-17). The post page and the
 * person page set it just before they close, and a comment's report sets it and stays; the one
 * overlay in app/(app)/_layout.tsx shows it over whatever page is showing then. The second look's
 * page sets its own line here as it goes back to Notifications: "Sent. We'll take another look."
 * (2026-11-19), or "Second look not sent. Please try again." when a send fails after the page was
 * left. The store clears itself after 2.5 seconds, and a new text restarts the timer. A
 * copy of lib/settings/notice.ts, kept apart because Settings prints any text its own store holds.
 */

export const REPORT_NOTICE_MS = 2500;

let current = '';
let timer: ReturnType<typeof setTimeout> | null = null;
const listeners = new Set<() => void>();

function emit(): void {
  listeners.forEach((listener) => listener());
}

/** Shows `text` for 2.5 seconds. An empty text clears the notice at once. */
export function setReportNotice(text: string): void {
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
    }, REPORT_NOTICE_MS);
  }
  emit();
}

/** The text now, '' when there is none. */
export function getReportNotice(): string {
  return current;
}

export function subscribeReportNotice(listener: () => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

/** The notice text for the overlay: '' when there is none. */
export function useReportNotice(): string {
  return useSyncExternalStore(subscribeReportNotice, getReportNotice, getReportNotice);
}

import { useCallback, useEffect, useRef, useState } from 'react';
import { useFocusEffect } from 'expo-router';
import { getSummary, type MeSummary } from '@/lib/api/summary';
import { useSession } from '@/lib/session/store';

export type ProfileSummaryState = {
  /** True until the first answer or the first failure of this visit settles. */
  loading: boolean;
  /**
   * The last good summary, or null when none has arrived (the server is not live yet, the phone
   * is offline, or the first answer is still on its way). The page shows zeros and empty lines
   * for null: it never shows an error screen.
   */
  summary: MeSummary | null;
};

type Held = ProfileSummaryState & { ownerId: string | null };

/**
 * Loads `GET /me/summary` for the Profile page (04-UI-SPEC 4.5): when the page mounts and every
 * time it regains focus, so a post, a delete or a rename shows at once.
 *
 *  - A failed reload keeps what is on screen (the last good summary stays).
 *  - A failure with nothing on screen settles to `summary: null`, which the page draws as zeros.
 *  - Answers that arrive out of order are dropped: only the newest request counts.
 *  - What belongs to another account is never shown (the data carries the id of its owner).
 */
export function useProfileSummary(): ProfileSummaryState {
  const { user } = useSession();
  const userId = user?.id ?? null;
  const [held, setHeld] = useState<Held>({ ownerId: userId, loading: true, summary: null });
  const newest = useRef(0);

  const load = useCallback(async () => {
    if (userId === null) return;
    newest.current += 1;
    const mine = newest.current;
    try {
      const summary = await getSummary();
      if (mine !== newest.current) return;
      setHeld({ ownerId: userId, loading: false, summary });
    } catch {
      if (mine !== newest.current) return;
      // Keep the last good answer of this same person; otherwise nothing is known.
      setHeld((previous) => ({
        ownerId: userId,
        loading: false,
        summary: previous.ownerId === userId ? previous.summary : null,
      }));
    }
  }, [userId]);

  useFocusEffect(
    useCallback(() => {
      void load();
    }, [load])
  );

  // Leaving for good: an answer that is still on its way is dropped.
  useEffect(
    () => () => {
      newest.current += 1;
    },
    []
  );

  if (held.ownerId !== userId) return { loading: true, summary: null };
  return { loading: held.loading, summary: held.summary };
}

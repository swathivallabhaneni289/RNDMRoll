import { useCallback, useEffect, useRef, useState } from 'react';
import { api } from '@/lib/api/client';
import type { UsernameAvailability } from '@/lib/api/types';
import {
  USERNAME_CHECK_FAILED_MESSAGE,
  USERNAME_PATTERN,
  USERNAME_TAKEN_MESSAGE,
  USERNAME_TAKEN_NO_ALTERNATES_MESSAGE,
  usernameProblem,
} from '@/lib/settings/rules';

/**
 * The live "is this username free" check of the Username page (plan 02.1-02, decision 4). It
 * repeats the rules of the form in components/profile/ProfileForm.tsx, which is not refactored to
 * use it (unify later): lower case as typed, nothing is checked while the box is empty or holds
 * the stored name, the format message at once, a 400ms debounce, then GET /usernames/available
 * without a token, and an answer for a name the box no longer holds is ignored.
 */

export type UsernameCheckStatus = 'idle' | 'checking' | 'available' | 'taken';

const DEBOUNCE_MS = 400;

// Without alternates the "Try one of these:" lead-in would point at nothing.
function takenMessage(alternates: string[]): string {
  return alternates.length > 0 ? USERNAME_TAKEN_MESSAGE : USERNAME_TAKEN_NO_ALTERNATES_MESSAGE;
}

export function useUsernameCheck(stored: string) {
  const [value, setValueState] = useState(stored);
  const [status, setStatus] = useState<UsernameCheckStatus>('idle');
  const [error, setError] = useState('');
  const [alternates, setAlternates] = useState<string[]>([]);

  // Refs, so a late answer can tell it is stale and a timer can be cleared from anywhere.
  const latestRef = useRef(value);
  const mountedRef = useRef(true);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
      if (timerRef.current !== null) {
        clearTimeout(timerRef.current);
        timerRef.current = null;
      }
    };
  }, []);

  async function check(candidate: string) {
    try {
      const result = await api.get<UsernameAvailability>(
        `/usernames/available?username=${encodeURIComponent(candidate)}`,
        { auth: false }
      );
      if (!mountedRef.current || latestRef.current !== candidate) return; // stale: the box moved on
      if (result.available) {
        setStatus('available');
        setAlternates([]);
        setError('');
      } else {
        const names = result.alternates ?? [];
        setStatus('taken');
        setAlternates(names);
        setError(takenMessage(names));
      }
    } catch {
      if (!mountedRef.current || latestRef.current !== candidate) return;
      // Save stays off: only an answer of "free" turns it on.
      setStatus('idle');
      setAlternates([]);
      setError(USERNAME_CHECK_FAILED_MESSAGE);
    }
  }

  useEffect(() => {
    latestRef.current = value;
    if (timerRef.current !== null) {
      clearTimeout(timerRef.current);
      timerRef.current = null;
    }

    if (value === '' || value === stored) {
      // Nothing typed, or back to the stored name: nothing to check, never "taken" against itself.
      setStatus('idle');
      setError('');
      setAlternates([]);
      return;
    }

    const problem = usernameProblem(value, stored);
    if (problem !== '') {
      setStatus('idle');
      setError(problem);
      setAlternates([]);
      return;
    }

    setError('');
    setAlternates([]);
    setStatus('checking');
    timerRef.current = setTimeout(() => {
      timerRef.current = null;
      void check(value);
    }, DEBOUNCE_MS);

    return () => {
      if (timerRef.current !== null) {
        clearTimeout(timerRef.current);
        timerRef.current = null;
      }
    };
    // `check` only reads refs and setters, so it never goes stale.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [value, stored]);

  const setValue = useCallback((raw: string) => {
    setValueState(raw.toLowerCase());
  }, []);

  /** A tap on one of the alternates puts it in the box (the check then runs on it). */
  const pick = useCallback((alternate: string) => {
    setValueState(alternate.toLowerCase());
  }, []);

  /**
   * The server said the name is taken at save time (a 409 after the check said free): show the
   * same taken state with the fresh alternates from that answer, and Save goes off again.
   */
  const markTaken = useCallback((suggestions: string[]) => {
    setStatus('taken');
    setAlternates(suggestions);
    setError(takenMessage(suggestions));
  }, []);

  const canSave = value !== stored && USERNAME_PATTERN.test(value) && status === 'available';

  return { value, setValue, status, error, alternates, canSave, pick, markTaken };
}

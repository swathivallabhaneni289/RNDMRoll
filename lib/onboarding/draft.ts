import { useEffect, useState } from 'react';

/**
 * A small module-scoped store for onboarding-time values that are not
 * credentials and must never touch the OS keychain: the email address to
 * display on the verify-email waiting screen, and the name Apple hands back
 * on a user's first authorization (see RESEARCH.md Pitfall 1b below).
 *
 * Deliberately in-memory only, not AsyncStorage-backed like intro-seen.ts:
 * this draft only needs to survive in-process navigation across the D-05
 * sequence, not a killed-and-relaunched app, and the address/name it carries
 * should not linger on disk once onboarding finishes.
 *
 * `suggestedName` exists because AppleAuthentication.signInAsync() returns
 * `fullName` and `email` only on a user's very first authorization for a
 * given Apple ID + bundle ID pair, and returns null on every subsequent
 * call. The value must be captured at that moment (lib/auth/social.ts does
 * this) and carried forward into the D-05 step 3 "Create your profile"
 * screen (plan 01-12), which prefills its name field from here.
 */

export interface OnboardingDraft {
  email?: string;
  suggestedName?: string;
  provider?: 'email' | 'apple' | 'google';
}

let draft: OnboardingDraft = {};

type Listener = (next: OnboardingDraft) => void;
const listeners = new Set<Listener>();

function notify(): void {
  for (const listener of listeners) {
    listener(draft);
  }
}

export function setDraft(patch: Partial<OnboardingDraft>): void {
  draft = { ...draft, ...patch };
  notify();
}

export function getDraft(): OnboardingDraft {
  return draft;
}

export function clearDraft(): void {
  draft = {};
  notify();
}

/** Re-renders the calling component whenever setDraft/clearDraft is called anywhere. */
export function useDraft(): OnboardingDraft {
  const [value, setValue] = useState<OnboardingDraft>(draft);

  useEffect(() => {
    listeners.add(setValue);
    return () => {
      listeners.delete(setValue);
    };
  }, []);

  return value;
}

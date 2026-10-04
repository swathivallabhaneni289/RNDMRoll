import AsyncStorage from '@react-native-async-storage/async-storage';
import { useEffect, useState } from 'react';

/**
 * Non-credential flag recording that the pre-signup Welcome page has already been
 * shown to this device.
 *
 * This is deliberately NOT a credential and must never go in the OS keychain:
 * RESEARCH.md's Don't Hand-Roll table reserves the keychain-backed store for
 * credentials specifically, and lib/session/store.ts is the only module in the
 * repository permitted to import that keychain module. AsyncStorage is the correct,
 * plain on-device store for a flag this low-stakes, and it must also be readable
 * with no session present, since its entire job is deciding what an unauthenticated
 * cold start opens on.
 */

const INTRO_SEEN_KEY = 'rndmroll.intro_seen';

/**
 * Returns false on any read failure. Failing to record the flag must degrade into
 * showing the intro once more rather than crashing a launch.
 */
export async function hasSeenIntro(): Promise<boolean> {
  try {
    const value = await AsyncStorage.getItem(INTRO_SEEN_KEY);
    return value === 'true';
  } catch {
    return false;
  }
}

/** Idempotent, and swallows write failures for the same reason as above. */
export async function markIntroSeen(): Promise<void> {
  try {
    await AsyncStorage.setItem(INTRO_SEEN_KEY, 'true');
  } catch {
    // Swallowed: worst case is the intro shows again next launch.
  }
}

/**
 * `seenIntro` is null until the AsyncStorage read resolves. `resolved` becomes true
 * once that read has settled (success or failure), so callers (the launch gate in
 * app/_layout.tsx, and the initial-route decision in app/(auth)/_layout.tsx) can
 * tell "not yet known" apart from "known false".
 */
export function useIntroSeen(): { seenIntro: boolean | null; resolved: boolean } {
  const [seenIntro, setSeenIntro] = useState<boolean | null>(null);
  const [resolved, setResolved] = useState(false);

  useEffect(() => {
    let cancelled = false;

    hasSeenIntro().then((value) => {
      if (cancelled) return;
      setSeenIntro(value);
      setResolved(true);
    });

    return () => {
      cancelled = true;
    };
  }, []);

  return { seenIntro, resolved };
}

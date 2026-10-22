import AsyncStorage from '@react-native-async-storage/async-storage';

/**
 * Forgot password once counted wrong codes on the phone and kept that count in AsyncStorage under
 * this key. The phone's count could not stay in step with the server's, so the page no longer keeps
 * one (app/(auth)/forgot-password.tsx); whatever an earlier build left is removed, once.
 */
const OLD_RESET_TRIES_KEY = 'rndmroll.reset_tries';
/** Written once the old count is gone: a later start sees it and removes nothing. */
const CLEARED_KEY = 'rndmroll.reset_tries_cleared';

/**
 * Removes the old count, once: after a start that removed it, a start only reads the mark. Quiet:
 * never throws, never waited on. A removal that fails writes no mark, so the next start tries
 * again.
 */
export function clearResetTries(): void {
  void (async () => {
    try {
      if ((await AsyncStorage.getItem(CLEARED_KEY)) !== null) return;
      await AsyncStorage.removeItem(OLD_RESET_TRIES_KEY);
      await AsyncStorage.setItem(CLEARED_KEY, '1');
    } catch {
      // Nothing to do: the next start tries again.
    }
  })();
}

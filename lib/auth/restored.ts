import { Alert } from 'react-native';
import type { AuthResult } from '@/lib/api/types';

export const RESTORED_TITLE = 'Welcome back';
export const RESTORED_MESSAGE = "Your account won't be deleted.";

/**
 * Logging back in within 30 days of Delete account cancels the deletion (developer, 2026-10-25,
 * like Instagram), and the server says so with `restored: true` on the answer of Log in. Called
 * once that answer has signed in, this tells the person in the phone's own alert. Any other answer
 * shows nothing. A Forgot password reset cancels it too, and signs nobody in, so its done step
 * shows RESTORED_MESSAGE itself.
 */
export function sayIfRestored(result: AuthResult): void {
  if (result.restored === true) Alert.alert(RESTORED_TITLE, RESTORED_MESSAGE);
}

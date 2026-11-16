import { AccessibilityInfo } from 'react-native';
import { spokenProblem } from '@/lib/forms/rules';

/**
 * Says a form's problem with VoiceOver, the way Posted and Saved are said. A polite live region
 * cannot: accessibilityLiveRegion is Android only in this React Native, and the app is iPhone only.
 * A form that shows its messages on a tap of its grey button says the first one:
 * announceProblem("Usernames can't have dots.", 'Username') says "Username. Usernames can't have
 * dots.". An empty message says nothing. Without VoiceOver on, nothing happens.
 */
export function announceProblem(text: string, label?: string): void {
  const spoken = spokenProblem(text, label);
  if (spoken !== '') AccessibilityInfo.announceForAccessibility(spoken);
}

import { LegalScreen } from '@/components/legal/LegalScreen';
import { PRIVACY } from '@/lib/legal/documents';

/**
 * The Privacy Policy for people who are not in the app yet, opened from the sign-up page. All the
 * behaviour lives in components/legal/LegalScreen.tsx.
 */
export default function PrivacyScreen() {
  return <LegalScreen document={PRIVACY} fallback="/welcome" />;
}

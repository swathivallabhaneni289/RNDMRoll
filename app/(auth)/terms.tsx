import { LegalScreen } from '@/components/legal/LegalScreen';
import { TERMS } from '@/lib/legal/documents';

/**
 * The Terms of Use for people who are not in the app yet, opened from the sign-up page. All the
 * behaviour lives in components/legal/LegalScreen.tsx.
 */
export default function TermsScreen() {
  return <LegalScreen document={TERMS} fallback="/welcome" />;
}

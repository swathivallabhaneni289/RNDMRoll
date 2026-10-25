import { LegalScreen } from '@/components/legal/LegalScreen';
import { TERMS } from '@/lib/legal/documents';

/**
 * The Terms of Use, pushed over Settings with the bottom bar showing. All the behaviour lives in
 * components/legal/LegalScreen.tsx.
 */
export default function TermsRoute() {
  return <LegalScreen document={TERMS} fallback="/(app)/profile/settings" edges={['top', 'left', 'right']} />;
}

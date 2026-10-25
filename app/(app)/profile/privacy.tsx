import { LegalScreen } from '@/components/legal/LegalScreen';
import { PRIVACY } from '@/lib/legal/documents';

/**
 * The Privacy Policy, pushed over Settings with the bottom bar showing. All the behaviour lives in
 * components/legal/LegalScreen.tsx.
 */
export default function PrivacyRoute() {
  return <LegalScreen document={PRIVACY} fallback="/(app)/profile/settings" edges={['top', 'left', 'right']} />;
}

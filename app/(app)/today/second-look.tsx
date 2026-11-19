import { useLocalSearchParams } from 'expo-router';
import { SecondLookScreen } from '@/components/notifications/SecondLookScreen';

/**
 * The page to ask for a second look at a removed post or comment (developer, 2026-11-19), pushed
 * from its line in Notifications. The route carries only the removal's id. No bottom bar.
 */
export default function SecondLookRoute() {
  const params = useLocalSearchParams<{ removal?: string | string[] }>();
  const removal = Array.isArray(params.removal) ? params.removal[0] : params.removal;
  return <SecondLookScreen removal={removal ?? ''} />;
}

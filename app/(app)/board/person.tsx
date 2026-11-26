import { PersonRoute } from '@/components/people/PersonRoute';

/**
 * Someone's profile page, opened by a tap on their photo or name in this tab, or by an invite
 * link that came while signed in (`username` in place of `id`, see app/+native-intent.tsx).
 */
export default function BoardPersonRoute() {
  return <PersonRoute home="/(app)/board" />;
}

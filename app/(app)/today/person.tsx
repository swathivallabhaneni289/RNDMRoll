import { PersonRoute } from '@/components/people/PersonRoute';

/**
 * Someone's profile page, opened by a tap on their photo or name in this tab, by a push, or by an
 * invite link kept from before signing in (`username` in place of `id`, lib/people/invite.ts).
 */
export default function TodayPersonRoute() {
  return <PersonRoute home="/(app)/today" />;
}

import { TodayScreen } from '@/components/today/TodayScreen';
import { useOpenInvite } from '@/lib/people/invite';

/**
 * The signed-in landing screen: today's spin. All behaviour lives in components/today. An invite
 * link kept from before signing in opens its page over this one, once (lib/people/invite.ts).
 */
export default function TodayRoute() {
  useOpenInvite();
  return <TodayScreen />;
}

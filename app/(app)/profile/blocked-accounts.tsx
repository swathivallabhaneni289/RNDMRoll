import { BlockedAccountsScreen } from '@/components/settings/BlockedAccountsScreen';

/**
 * Settings > Blocked accounts, pushed from the Blocked accounts row of Settings (decision D-16).
 * The page itself lives in components/settings/BlockedAccountsScreen.tsx.
 */
export default function BlockedAccountsRoute() {
  return <BlockedAccountsScreen />;
}

import { NotificationsScreen } from '@/components/today/NotificationsScreen';

/** The page behind the Board's bell. Back returns to the Board; its taps stay in this stack. */
export default function BoardNotificationsRoute() {
  return <NotificationsScreen place="board" />;
}

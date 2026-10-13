import { ProfileForm } from '@/components/profile/ProfileForm';
import { ProfileScreen } from '@/components/profile/ProfileScreen';
import { useSession } from '@/lib/session/store';

/**
 * The Profile tab, and the first-run route.
 *
 * Right after a sign-up it shows Phase 1's photo-and-bio page, once (the bottom bar is hidden on
 * it, and Continue or Skip for now end on Today). After that it is the Profile page of the
 * developer's picture (04-UI-SPEC section 4): the photo with the name and @username beside it, the
 * bio, the three figures, the tastes and the recent activity. The Edit button on that page pushes
 * `profile/edit` (Phase 1's form in a pushed page), and its gear pushes `profile/settings`.
 *
 * The page reads the session user directly, so a change made on a page pushed over it (the bio,
 * the Edit page) shows the moment it comes back into view; nothing here copies the user.
 */
export default function ProfileLandingScreen() {
  const { extrasPending } = useSession();
  if (extrasPending) return <ProfileForm key="extras" mode="extras" inTab />;
  return <ProfileScreen />;
}

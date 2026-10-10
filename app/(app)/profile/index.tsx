import { ProfileForm } from '@/components/profile/ProfileForm';
import { useSession } from '@/lib/session/store';

/**
 * The Profile tab, and the first-run route. Right after a sign-up it shows the photo-and-bio
 * page, once (the bottom bar is hidden on it, and Continue or Skip for now end on Today). After
 * that it is the edit page inside the tab. The key remounts the form when the page changes, so
 * nothing typed on one page can carry over to the other. `inTab` only matters in edit mode.
 */
export default function ProfileLandingScreen() {
  const { extrasPending } = useSession();
  const mode = extrasPending ? 'extras' : 'edit';
  return <ProfileForm key={mode} mode={mode} inTab />;
}

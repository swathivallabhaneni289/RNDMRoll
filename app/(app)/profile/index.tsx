import { ProfileForm } from '@/components/profile/ProfileForm';
import { useSession } from '@/lib/session/store';

/**
 * The landing route. Right after a sign-up it shows the photo-and-bio page, once. After that,
 * and on every later launch, it shows the edit page (edit.tsx). The key remounts the form when
 * the page changes, so nothing typed on one page can carry over to the other.
 */
export default function ProfileLandingScreen() {
  const { extrasPending } = useSession();
  const mode = extrasPending ? 'extras' : 'edit';
  return <ProfileForm key={mode} mode={mode} />;
}

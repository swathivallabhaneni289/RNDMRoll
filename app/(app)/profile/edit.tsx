import { ProfileForm } from '@/components/profile/ProfileForm';

/**
 * The Edit page: Phase 1's "Make it yours." form in edit mode, pushed over the Profile page by its
 * Edit button, with a Back control: photo (with "Edit your picture"), name, username with its live
 * check, bio, "Save changes" (grey until something changes) and Log out. After a save it goes back
 * and the Profile page shows "Saved." All the behavior lives in components/profile/ProfileForm.tsx.
 */
export default function EditProfileScreen() {
  return <ProfileForm mode="edit" pushed />;
}

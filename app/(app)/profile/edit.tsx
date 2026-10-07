import { ProfileForm } from '@/components/profile/ProfileForm';

/**
 * The "Make it yours." page in edit mode: photo, name, username, bio, Save changes, Log out.
 * It is also the app's landing page (index.tsx re-exports it). All the behavior lives in
 * components/profile/ProfileForm.tsx.
 */
export default function EditProfileScreen() {
  return <ProfileForm mode="edit" />;
}

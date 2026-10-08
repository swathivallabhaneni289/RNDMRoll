import { ProfileForm } from '@/components/profile/ProfileForm';

/**
 * The "Make it yours." page in edit mode: photo, name, username, bio, Save changes, Log out.
 * It is also the app's landing page: index.tsx shows the same form, except for the one-time
 * photo-and-bio page right after sign-up. All the behavior lives in
 * components/profile/ProfileForm.tsx.
 */
export default function EditProfileScreen() {
  return <ProfileForm mode="edit" />;
}

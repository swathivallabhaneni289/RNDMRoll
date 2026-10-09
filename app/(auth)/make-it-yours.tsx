import { ProfileForm } from '@/components/profile/ProfileForm';

/**
 * The "Make it yours." page for people who are not in the app yet: the sign-up form. All the
 * behavior lives in components/profile/ProfileForm.tsx.
 */
export default function MakeItYoursScreen() {
  return <ProfileForm mode="signup" />;
}

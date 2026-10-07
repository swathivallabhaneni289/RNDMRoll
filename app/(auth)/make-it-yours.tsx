import { useState } from 'react';
import { ProfileForm, type ProfileFormMode } from '@/components/profile/ProfileForm';
import { useSession } from '@/lib/session/store';

/**
 * The "Make it yours." page for people who are not in the app yet: sign-up when signed out,
 * finish mode when a signed-in account (Apple or Google) is still incomplete. The mode is
 * chosen ONCE, on mount, and never recomputed: signIn and signOut change the session while
 * the screen is still up, and the form must not show anything new during that swap.
 */
export default function MakeItYoursScreen() {
  const { status, user } = useSession();
  const [mode] = useState<ProfileFormMode>(() =>
    status === 'authenticated' && user?.onboarding_complete === false ? 'finish' : 'signup'
  );
  return <ProfileForm mode={mode} />;
}

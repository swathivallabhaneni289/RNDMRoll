import { useCallback, useRef, useState } from 'react';
import { useFocusEffect } from 'expo-router';
import { ProfileForm } from '@/components/profile/ProfileForm';
import { useSession } from '@/lib/session/store';

type SessionUser = ReturnType<typeof useSession>['user'];

/** The four things the form copies from the session when it mounts, as one comparable text. */
function shownFields(user: SessionUser): string {
  return JSON.stringify([user?.name ?? '', user?.username ?? '', user?.bio ?? '', user?.avatar_url ?? '']);
}

/**
 * The Profile tab, and the first-run route. Right after a sign-up it shows the photo-and-bio
 * page, once (the bottom bar is hidden on it, and Continue or Skip for now end on Today). After
 * that it is the edit page inside the tab. The key remounts the form when the page changes, so
 * nothing typed on one page can carry over to the other. `inTab` only matters in edit mode.
 *
 * Settings is pushed over this page, which stays mounted underneath, and it can change the
 * username or the bio while the form (which copies them once, when it mounts) is out of sight. So
 * when this page is focused again and what the form shows is no longer what the session holds, the
 * form is mounted afresh (the generation in the key). Nothing is remounted while the page stays
 * focused: the form's own Save changes keeps its "Saved." note, and an unfinished edit survives a
 * visit to Settings that changed none of these four fields.
 */
export default function ProfileLandingScreen() {
  const { extrasPending, user } = useSession();
  const mode = extrasPending ? 'extras' : 'edit';

  const shownNow = useRef('');
  shownNow.current = shownFields(user);
  const shownAtBlur = useRef<string | null>(null);
  const [generation, setGeneration] = useState(0);

  useFocusEffect(
    useCallback(() => {
      if (shownAtBlur.current !== null && shownAtBlur.current !== shownNow.current) {
        setGeneration((value) => value + 1);
      }
      return () => {
        shownAtBlur.current = shownNow.current;
      };
    }, [])
  );

  return <ProfileForm key={`${mode}-${generation}`} mode={mode} inTab />;
}

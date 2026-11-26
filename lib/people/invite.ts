import AsyncStorage from '@react-native-async-storage/async-storage';
import { useCallback } from 'react';
import { useFocusEffect, useRouter } from 'expo-router';
import { createInviteStore } from '@/lib/people/inviteLink';
import { useSession } from '@/lib/session/store';

/**
 * The app's kept invite link (lib/people/inviteLink.ts has the rules). AsyncStorage, as for the
 * Welcome flag (lib/onboarding/intro-seen.ts): a username is not a credential, so not the keychain.
 * app/+native-intent.tsx keeps a link that comes while signed out, app/_layout.tsx forgets it as the
 * session changes, the sign-in pages and Find your friends make it ready, and the Spin page opens
 * it (useOpenInvite).
 */
export const invites = createInviteStore(AsyncStorage);

/**
 * The Spin page: a kept invite that is ready opens the inviter's page once, over it, so Back comes
 * back here (PRD known issue 1, 2026-11-26). Read on focus, so it opens as the person lands here
 * after Log in, or after Find your friends at the end of a sign-up.
 */
export function useOpenInvite(): void {
  const router = useRouter();
  const { user } = useSession();
  const own = user?.username ?? null;

  useFocusEffect(
    useCallback(() => {
      let alive = true;
      void invites.ready().then(() => {
        // Left before the kept invite was read: the next visit opens it.
        if (!alive) return;
        const username = invites.take(own);
        if (username !== null) router.push({ pathname: '/(app)/today/person', params: { username } });
      });
      return () => {
        alive = false;
      };
    }, [own, router]),
  );
}

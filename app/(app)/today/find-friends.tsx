import { SearchScreen } from '@/components/people/SearchScreen';

/**
 * Find your friends, shown once right after sign-up (after the photo-and-bio page): search,
 * Invite friends and Suggested for you, then Skip for now (Continue once the person follows someone
 * or sends the link) goes on to the Spin page. No bottom bar.
 */
export default function FindFriendsRoute() {
  return <SearchScreen welcome />;
}

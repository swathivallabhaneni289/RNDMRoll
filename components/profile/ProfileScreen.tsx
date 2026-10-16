import { useEffect } from 'react';
import { AccessibilityInfo, View } from 'react-native';
import { useRouter } from 'expo-router';
import { AppText } from '@/components/ui/AppText';
import { Chip } from '@/components/ui/Chip';
import { Screen } from '@/components/ui/Screen';
import { SettingsGear } from '@/components/settings/SettingsGear';
import { RecentActivity } from '@/components/profile/RecentActivity';
import { SizedAvatar } from '@/components/profile/SizedAvatar';
import { StatsRow } from '@/components/profile/StatsRow';
import { useProfileSummary } from '@/components/profile/useProfileSummary';
import type { MeSummary } from '@/lib/api/summary';
import { useSettingsNotice } from '@/lib/settings/notice';
import { useSession } from '@/lib/session/store';
import { minTouchTarget, space } from '@/lib/theme/tokens';

/** Drawn for a person with no answer from the server (not live yet, offline, or nothing posted). */
const EMPTY_SUMMARY: MeSummary = { spins: 0, week_streak: 0, friends: 0, tastes: [], recent: [] };

const AVATAR_SIZE = 72;

/** The only notice this page shows: the Edit page puts it into Settings' notice store after a save. */
const SAVED_NOTICE = 'Saved.';

export type ProfileViewProps = {
  name: string;
  username: string;
  /** The bio as saved, shown as plain text; it is edited on the Edit page. */
  bio: string;
  avatarUri: string | null;
  /** True until the first answer (or failure) of this visit settles. */
  loading: boolean;
  /** The last good summary, or null for none (the page then draws zeros and empty lines). */
  summary: MeSummary | null;
  /** The Edit page has just saved. */
  saved: boolean;
  onEdit: () => void;
  /** Opens a recent post on the entry page; without it the recent rows are plain. */
  onOpenEntry?: (entryId: string) => void;
};

/**
 * The Profile page as the developer's picture draws it (04-UI-SPEC section 4), built from props so
 * the same drawing serves the live page and any check of it.
 *
 * Top to bottom: a slim row with the Settings gear at the right (and 'Saved.' at the left for a few
 * seconds after the Edit page saves); the photo in a circle with the name beside it and the
 * @username under the name, and the outlined 'Edit' button at the right; the bio as plain text (it is edited on the Edit page);
 * a dark rule and the three figures; the recent activity.
 *
 * Differences from the picture, on purpose: the gear and the bio (asked for in words, the picture
 * draws neither), 'Spins' for 'Rolls', and the section labels at the app's Label size.
 */
export function ProfileView({
  name,
  username,
  bio,
  avatarUri,
  loading,
  summary,
  saved,
  onEdit,
  onOpenEntry,
}: ProfileViewProps) {
  const shown = summary ?? EMPTY_SUMMARY;

  // iOS does not read a live region by itself: say 'Saved.' once, when it appears.
  useEffect(() => {
    if (saved) AccessibilityInfo.announceForAccessibility(SAVED_NOTICE);
  }, [saved]);

  return (
    <Screen edges={['top', 'left', 'right']}>
      <View style={{ paddingBottom: space.xl }}>
        {/* A slim row above the photo: 'Saved.' on the left for 3 seconds, the Settings gear on the right. */}
        <View
          style={{
            minHeight: minTouchTarget,
            flexDirection: 'row',
            alignItems: 'center',
            justifyContent: 'space-between',
          }}
        >
          <View accessibilityLiveRegion="polite" style={{ flex: 1 }}>
            {saved ? (
              <AppText role="label" tone="muted">
                {SAVED_NOTICE}
              </AppText>
            ) : null}
          </View>
          <SettingsGear />
        </View>

        <View style={{ flexDirection: 'row', alignItems: 'center' }}>
          <SizedAvatar size={AVATAR_SIZE} name={name} uri={avatarUri} placeholder="camera" />
          <View style={{ flex: 1, marginLeft: space.md }}>
            {name !== '' ? (
              <AppText role="note" numberOfLines={2} ellipsizeMode="tail" accessibilityRole="header">
                {name}
              </AppText>
            ) : null}
            {username !== '' ? (
              <View style={{ marginTop: name !== '' ? space.xs : 0 }}>
                <AppText role="body" tone="muted" numberOfLines={1} ellipsizeMode="tail">
                  {`@${username}`}
                </AppText>
              </View>
            ) : null}
          </View>
          <View style={{ marginLeft: space.md }}>
            <Chip label="Edit" tone="strong" onPress={onEdit} accessibilityLabel="Edit profile" />
          </View>
        </View>

        {/* The bio runs the full width under the photo row, so a long one has room (04-UI-SPEC 4.1). */}
        {bio !== '' ? (
          <View style={{ marginTop: space.sm }}>
            <AppText role="body">{bio}</AppText>
          </View>
        ) : null}

        {loading ? (
          <View style={{ marginTop: space.lg }}>
            <AppText role="label" tone="muted">
              Loading your profile.
            </AppText>
          </View>
        ) : (
          <>
            <View style={{ marginTop: space.lg }}>
              <StatsRow spins={shown.spins} weekStreak={shown.week_streak} friends={shown.friends} />
            </View>
            <View style={{ marginTop: space.lg }}>
              <RecentActivity recent={shown.recent} onOpenEntry={onOpenEntry} />
            </View>
          </>
        )}
      </View>
    </Screen>
  );
}

/**
 * The Profile tab after first run: the session user for the header, `GET /me/summary` for the
 * figures and lists, and the 'Saved.' notice from the Edit page. A summary the server cannot give
 * yet (a 404, or no connection) shows zeros and empty lines, never an error screen.
 *
 * A recent row opens the entry page inside the Profile stack (`/(app)/profile/entry` with
 * `from: 'profile'`), so Back reads 'Profile'. There is no tastes section: an average per
 * category was dropped.
 */
export function ProfileScreen() {
  const router = useRouter();
  const { user } = useSession();
  const { loading, summary } = useProfileSummary();
  const notice = useSettingsNotice();

  return (
    <ProfileView
      name={user?.name ?? ''}
      username={user?.username ?? ''}
      bio={user?.bio ?? ''}
      avatarUri={user?.avatar_url ?? null}
      loading={loading}
      summary={summary}
      saved={notice === SAVED_NOTICE}
      onEdit={() => router.push('/(app)/profile/edit')}
      onOpenEntry={(id) => router.push({ pathname: '/(app)/profile/entry', params: { id, from: 'profile' } })}
    />
  );
}

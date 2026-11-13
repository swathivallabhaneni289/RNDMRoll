import { useCallback, useRef, useState } from 'react';
import { View } from 'react-native';
import { useFocusEffect } from 'expo-router';
import { AppText } from '@/components/ui/AppText';
import { Chip } from '@/components/ui/Chip';
import { Screen } from '@/components/ui/Screen';
import { BackRow } from '@/components/settings/BackRow';
import { SizedAvatar } from '@/components/profile/SizedAvatar';
import { StatusMessage } from '@/components/today/StatusMessage';
import { getBlocks, unblock, type BlockedAccount } from '@/lib/api/account';
import { ApiError } from '@/lib/api/client';
import { useForegroundReload } from '@/lib/api/useForegroundReload';
import { GENERIC_MESSAGE } from '@/lib/settings/rules';
import { color, space } from '@/lib/theme/tokens';

const AVATAR = 44;

function Row({
  account,
  busy,
  error,
  onUnblock,
}: {
  account: BlockedAccount;
  busy: boolean;
  error: string;
  onUnblock: () => void;
}) {
  const { user } = account;
  const name = user.name ?? '';
  const handle = user.username ? `@${user.username}` : '';
  const spokenName = name !== '' ? name : handle;

  return (
    <View style={{ borderBottomWidth: 1, borderBottomColor: color.divider }}>
      <View style={{ flexDirection: 'row', alignItems: 'center', paddingVertical: space.sm }}>
        <View accessibilityElementsHidden importantForAccessibility="no-hide-descendants">
          <SizedAvatar size={AVATAR} name={name !== '' ? name : (user.username ?? '')} uri={user.photo_url} placeholder="initials" />
        </View>
        <View
          accessible
          accessibilityLabel={name !== '' && handle !== '' ? `${name}, ${handle}` : spokenName}
          style={{ flex: 1, marginLeft: space.md, marginRight: space.md }}
        >
          {name !== '' ? (
            <AppText role="body" numberOfLines={1} ellipsizeMode="tail">
              {name}
            </AppText>
          ) : null}
          {handle !== '' ? (
            <AppText role="label" tone="muted" numberOfLines={1} ellipsizeMode="tail">
              {handle}
            </AppText>
          ) : null}
        </View>
        {/* An outlined button like the person page's, never underlined text (developer, 2026-10-18). */}
        <Chip
          label="Unblock"
          tone="outline"
          onPress={busy ? undefined : onUnblock}
          accessibilityLabel={`Unblock ${spokenName}`}
          accessibilityState={{ disabled: busy }}
        />
      </View>
      {error !== '' ? (
        <View accessibilityLiveRegion="polite" style={{ paddingBottom: space.sm }}>
          <AppText role="label" tone="default">
            {error}
          </AppText>
        </View>
      ) : null}
    </View>
  );
}

/**
 * Settings > Blocked accounts (decision D-16): the people the person has blocked, each with a
 * round photo, the name, the @username and an outlined 'Unblock' button. Blocking itself is on the
 * person page (its three dots); here there is only the list and Unblock.
 *
 * The list is read on every focus, and on a return from the background once 20 minutes old (its
 * photo links stop working within an hour). A first read that fails shows the plain line with Try
 * again, never "You haven't blocked anyone."; a later read that fails keeps the list on screen. A
 * failed Unblock keeps the row and shows one line under it.
 */
export function BlockedAccountsScreen() {
  // null until the first answer, so the empty sentence does not flash and never stands in for a
  // read that failed.
  const [items, setItems] = useState<BlockedAccount[] | null>(null);
  // Why the last read failed; shown only while there is no list yet.
  const [failure, setFailure] = useState<'network' | 'generic' | null>(null);
  const [busyId, setBusyId] = useState<string | null>(null);
  const [errors, setErrors] = useState<Record<string, string>>({});
  // Only the newest read is used, and an Unblock that went through drops a read begun before it:
  // that answer still holds the account just unblocked.
  const seqRef = useRef(0);

  const { markLoaded } = useForegroundReload(() => void load());

  const load = useCallback(async () => {
    const seq = ++seqRef.current;
    try {
      const next = await getBlocks();
      if (seq !== seqRef.current) return;
      setItems(next);
      setFailure(null);
      markLoaded();
    } catch (err) {
      if (seq !== seqRef.current) return;
      setFailure(err instanceof ApiError && err.code === 'network_unavailable' ? 'network' : 'generic');
    }
  }, [markLoaded]);

  useFocusEffect(
    useCallback(() => {
      void load();
    }, [load])
  );

  async function handleUnblock(userId: string) {
    if (busyId !== null) return;
    setBusyId(userId);
    setErrors((current) => ({ ...current, [userId]: '' }));
    try {
      await unblock(userId);
      seqRef.current += 1;
      setItems((current) => (current ?? []).filter((account) => account.user.id !== userId));
    } catch (err) {
      setErrors((current) => ({
        ...current,
        [userId]: err instanceof ApiError ? err.userMessage : GENERIC_MESSAGE,
      }));
    } finally {
      setBusyId(null);
    }
  }

  return (
    <Screen edges={['top', 'left', 'right']}>
      <View style={{ paddingTop: space.lg, paddingBottom: space.xl }}>
        <BackRow />

        <View style={{ marginTop: space.md }}>
          <AppText role="title" accessibilityRole="header">
            Blocked accounts
          </AppText>
        </View>

        {items === null ? (
          failure === null ? null : (
            <View style={{ marginTop: space.lg }}>
              <StatusMessage kind={failure} onRetry={() => void load()} />
            </View>
          )
        ) : items.length === 0 ? (
          <View accessibilityLiveRegion="polite" style={{ marginTop: space.lg }}>
            <AppText role="body" tone="muted">
              You haven't blocked anyone.
            </AppText>
          </View>
        ) : (
          <View style={{ marginTop: space.lg, borderTopWidth: 1, borderTopColor: color.divider }}>
            {items.map((account) => (
              <Row
                key={account.user.id}
                account={account}
                busy={busyId === account.user.id}
                error={errors[account.user.id] ?? ''}
                onUnblock={() => void handleUnblock(account.user.id)}
              />
            ))}
          </View>
        )}
      </View>
    </Screen>
  );
}

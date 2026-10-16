import { useEffect, useState } from 'react';
import { Pressable, Text, View } from 'react-native';
import { AppText } from '@/components/ui/AppText';
import { Screen } from '@/components/ui/Screen';
import { BackRow } from '@/components/settings/BackRow';
import { SizedAvatar } from '@/components/profile/SizedAvatar';
import { getBlocks, unblock, type BlockedAccount } from '@/lib/api/account';
import { ApiError } from '@/lib/api/client';
import { GENERIC_MESSAGE } from '@/lib/settings/rules';
import { color, minTouchTarget, space, type } from '@/lib/theme/tokens';

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
        <Pressable
          onPress={onUnblock}
          disabled={busy}
          accessibilityRole="button"
          accessibilityLabel={`Unblock ${spokenName}`}
          accessibilityState={{ disabled: busy }}
          hitSlop={(minTouchTarget - type.label.lineHeight) / 2}
        >
          <Text
            style={{
              fontFamily: type.label.fontFamily,
              fontSize: type.label.fontSize,
              lineHeight: type.label.lineHeight,
              color: busy ? color.muted : color.ink,
              textDecorationLine: 'underline',
            }}
          >
            Unblock
          </Text>
        </Pressable>
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
 * round photo, the name, the @username and an 'Unblock' text link. Blocking itself comes with the
 * person pages of Phases 5 and 6; here there is only the list and Unblock.
 *
 * The server side is built after Phase 3's server, so `GET /me/blocks` may 404 for now. Then the
 * list is simply empty ("You haven't blocked anyone."); there is never an error screen. A failed
 * Unblock keeps the row and shows one red line under it.
 */
export function BlockedAccountsScreen() {
  // null until the first answer (or failure) settles, so the empty sentence does not flash.
  const [items, setItems] = useState<BlockedAccount[] | null>(null);
  const [busyId, setBusyId] = useState<string | null>(null);
  const [errors, setErrors] = useState<Record<string, string>>({});

  useEffect(() => {
    let alive = true;
    getBlocks()
      .then((loaded) => {
        if (alive) setItems(loaded);
      })
      .catch(() => {
        if (alive) setItems([]);
      });
    return () => {
      alive = false;
    };
  }, []);

  async function handleUnblock(userId: string) {
    if (busyId !== null) return;
    setBusyId(userId);
    setErrors((current) => ({ ...current, [userId]: '' }));
    try {
      await unblock(userId);
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

        {items === null ? null : items.length === 0 ? (
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

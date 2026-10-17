import { ReactNode, useCallback, useRef, useState } from 'react';
import { Alert, Pressable, Text, View } from 'react-native';
import { useFocusEffect, useRouter } from 'expo-router';
import { Ionicons } from '@expo/vector-icons';
import { SizedAvatar } from '@/components/profile/SizedAvatar';
import { StatusMessage } from '@/components/today/StatusMessage';
import { AppText } from '@/components/ui/AppText';
import { Chip } from '@/components/ui/Chip';
import { MenuSheet, type MenuItem } from '@/components/ui/MenuSheet';
import { Screen } from '@/components/ui/Screen';
import { blockUser } from '@/lib/api/account';
import { ApiError } from '@/lib/api/client';
import {
  acceptRequest,
  declineRequest,
  firstName,
  follow,
  getFollows,
  removeFollower,
  unfollow,
  type Follows,
  type Person,
} from '@/lib/api/social';
import { color, minTouchTarget, space, type } from '@/lib/theme/tokens';

const AVATAR = 40;
const ROW_MIN_HEIGHT = 56;
const GENERIC = 'Something went wrong. Please try again.';

type Place = 'profile' | 'board';
const PLACE_NAME: Record<Place, string> = { profile: 'Profile', board: 'Board' };
const PLACE_ROOT: Record<Place, '/(app)/profile' | '/(app)/board'> = { profile: '/(app)/profile', board: '/(app)/board' };

/** The two lists, each a tab. */
export type PeopleList = 'followers' | 'following';

/** What a row's sheet offers. */
type Menu = { person: Person; kind: 'following' | 'follower' | 'other' };

function messageFor(err: unknown): string {
  return err instanceof ApiError && (err.code === 'network_unavailable' || err.code === 'rate_limited') ? err.userMessage : GENERIC;
}

const capsText = {
  fontFamily: type.label.fontFamily,
  fontSize: type.label.fontSize,
  lineHeight: type.label.lineHeight,
  letterSpacing: 1.5,
  textTransform: 'uppercase',
  color: color.muted,
} as const;

function SectionLabel({ children }: { children: string }) {
  return (
    <Text accessibilityRole="header" style={capsText}>
      {children}
    </Text>
  );
}

/** One person: the round photo or initial, the name and @username, and the actions at the right. */
export function PersonRow({
  person,
  onPress,
  pressHint,
  children,
  first = false,
}: {
  person: Person;
  onPress?: () => void;
  pressHint?: string;
  children?: ReactNode;
  first?: boolean;
}) {
  const name = person.name !== '' ? person.name : person.username;
  const handle = person.username !== '' ? `@${person.username}` : '';
  const who = (
    <View style={{ flexDirection: 'row', alignItems: 'center', flex: 1 }}>
      <View accessibilityElementsHidden importantForAccessibility="no-hide-descendants">
        <SizedAvatar size={AVATAR} name={name} uri={person.photo_url} placeholder="initials" />
      </View>
      <View style={{ flex: 1, marginLeft: space.md }}>
        <AppText role="button" numberOfLines={1} ellipsizeMode="tail">
          {name}
        </AppText>
        {handle !== '' && handle !== `@${name}` ? (
          <AppText role="label" tone="muted" numberOfLines={1} ellipsizeMode="tail">
            {handle}
          </AppText>
        ) : null}
      </View>
    </View>
  );
  return (
    <View
      style={{
        minHeight: ROW_MIN_HEIGHT,
        flexDirection: 'row',
        alignItems: 'center',
        paddingVertical: space.sm,
        borderTopWidth: first ? 0 : 1,
        borderTopColor: color.divider,
      }}
    >
      {onPress ? (
        <Pressable
          onPress={onPress}
          accessibilityRole="button"
          accessibilityLabel={handle !== '' ? `${name}, ${handle}` : name}
          accessibilityHint={pressHint}
          style={{ flex: 1 }}
        >
          {who}
        </Pressable>
      ) : (
        <View accessible accessibilityLabel={handle !== '' ? `${name}, ${handle}` : name} style={{ flex: 1 }}>
          {who}
        </View>
      )}
      {children ? <View style={{ marginLeft: space.sm, flexDirection: 'row', alignItems: 'center' }}>{children}</View> : null}
    </View>
  );
}

/** The two lists as tabs, like Instagram's: the open one in ink with a dark line under it. */
function ListTabs({ value, onChange }: { value: PeopleList; onChange: (next: PeopleList) => void }) {
  const tabs: { key: PeopleList; label: string }[] = [
    { key: 'followers', label: 'Followers' },
    { key: 'following', label: 'Following' },
  ];
  return (
    <View accessibilityRole="tablist" style={{ flexDirection: 'row', borderBottomWidth: 1, borderBottomColor: color.divider }}>
      {tabs.map((tab) => {
        const on = tab.key === value;
        return (
          <Pressable
            key={tab.key}
            onPress={() => onChange(tab.key)}
            accessibilityRole="tab"
            accessibilityState={{ selected: on }}
            style={{
              flex: 1,
              minHeight: minTouchTarget,
              alignItems: 'center',
              justifyContent: 'center',
              borderBottomWidth: 2,
              borderBottomColor: on ? color.ink : 'transparent',
              marginBottom: -1,
            }}
          >
            <AppText role="button" tone={on ? undefined : 'muted'}>
              {tab.label}
            </AppText>
          </Pressable>
        );
      })}
    </View>
  );
}

function MoreButton({ name, onPress }: { name: string; onPress: () => void }) {
  return (
    <Pressable
      onPress={onPress}
      accessibilityRole="button"
      accessibilityLabel={`More for ${name}`}
      hitSlop={8}
      style={{ width: minTouchTarget, height: minTouchTarget, alignItems: 'flex-end', justifyContent: 'center' }}
    >
      <Ionicons name="ellipsis-horizontal" size={22} color={color.ink} />
    </Pressable>
  );
}

/**
 * People (05-FOLLOW-OVERRIDES section 7, the paper's Friends screen made one way). Opened from the
 * Followers and Following figures on Profile and from a follow notification, with a thin route in
 * each stack. Finding someone new lives in Search, behind the search icon on the Board (developer,
 * 2026-10-16), so this page only lists, under two tabs like Instagram's (developer, 2026-10-17):
 * Followers (any Requests on top with Accept and Decline; 'Follow back' on a follower you do not
 * follow, 'Requested' once asked; a row opens Remove follower and Block) and Following (a row
 * opens Unfollow and Block). `show` is the tab it opens on. Nothing here is red; a failure is one
 * quiet line in ink.
 */
export function PeopleScreen({ from, show = 'followers' }: { from: Place; show?: PeopleList }) {
  const router = useRouter();
  const [tab, setTab] = useState<PeopleList>(show);
  const [follows, setFollows] = useState<Follows | null>(null);
  const [status, setStatus] = useState<'loading' | 'ready' | 'offline'>('loading');
  const [failure, setFailure] = useState<'network' | 'generic'>('network');
  const [busy, setBusy] = useState<string | null>(null);
  const [message, setMessage] = useState('');
  const [menu, setMenu] = useState<Menu | null>(null);
  const busyRef = useRef(false);
  const followsRef = useRef<Follows | null>(null);

  const load = useCallback(async () => {
    try {
      const next = await getFollows();
      followsRef.current = next;
      setFollows(next);
      setStatus('ready');
    } catch (err) {
      if (followsRef.current !== null) return;
      setFailure(err instanceof ApiError && err.code === 'network_unavailable' ? 'network' : 'generic');
      setStatus('offline');
    }
  }, []);

  useFocusEffect(
    useCallback(() => {
      void load();
    }, [load])
  );

  function goBack() {
    if (router.canGoBack()) router.back();
    else router.replace(PLACE_ROOT[from]);
  }

  /** Runs one action at a time, then reads the lists again. */
  async function act(key: string, work: () => Promise<void>) {
    if (busyRef.current) return;
    busyRef.current = true;
    setBusy(key);
    setMessage('');
    try {
      await work();
    } catch (err) {
      setMessage(messageFor(err));
    } finally {
      busyRef.current = false;
      setBusy(null);
      void load();
    }
  }

  function doFollowBack(person: Person) {
    void act(`follow-${person.id}`, async () => {
      await follow(person.id);
    });
  }

  function doUnfollow(person: Person) {
    void act(`unfollow-${person.id}`, async () => {
      await unfollow(person.id);
    });
  }

  function doRemove(person: Person) {
    void act(`remove-${person.id}`, async () => {
      await removeFollower(person.id);
    });
  }

  function askBlock(person: Person) {
    const name = firstName(person);
    Alert.alert(`Block ${name}?`, "You will stop seeing each other's posts. They won't be told.", [
      { text: "Don't block", style: 'cancel' },
      {
        text: 'Block',
        onPress: () =>
          void act(`block-${person.id}`, async () => {
            await blockUser(person.id);
          }),
      },
    ]);
  }

  /** Close the sheet first, so an alert never opens under its dim cover. */
  function fromMenu(action: () => void) {
    return () => {
      setMenu(null);
      setTimeout(action, 0);
    };
  }

  const menuItems: MenuItem[] =
    menu === null
      ? []
      : menu.kind === 'other'
        ? [{ label: `Block ${firstName(menu.person)}`, onPress: fromMenu(() => askBlock(menu.person)) }]
        : menu.kind === 'following'
        ? [
            { label: 'Unfollow', onPress: fromMenu(() => doUnfollow(menu.person)) },
            { label: `Block ${firstName(menu.person)}`, onPress: fromMenu(() => askBlock(menu.person)) },
          ]
        : [
            { label: 'Remove follower', onPress: fromMenu(() => doRemove(menu.person)) },
            { label: `Block ${firstName(menu.person)}`, onPress: fromMenu(() => askBlock(menu.person)) },
          ];

  const followingIds = new Set((follows?.following ?? []).map((person) => person.id));
  const outgoing = (follows?.outgoing ?? []).filter((request) => !followingIds.has(request.person.id));
  const requestedIds = new Set(outgoing.map((request) => request.person.id));
  const requests = tab === 'followers' ? (follows?.incoming ?? []) : [];

  /** A follower you do not follow gets 'Follow back'; one you have asked shows 'Requested'. */
  function followerButton(person: Person) {
    const name = firstName(person);
    if (followingIds.has(person.id)) return null;
    if (requestedIds.has(person.id)) {
      return (
        <Chip
          label="Requested"
          tone="outline"
          onPress={busy !== null ? undefined : () => doUnfollow(person)}
          accessibilityLabel={`Requested. Cancel the request to ${name}`}
        />
      );
    }
    return (
      <Chip
        label="Follow back"
        tone="strong"
        onPress={busy !== null ? undefined : () => doFollowBack(person)}
        accessibilityLabel={`Follow back ${name}`}
      />
    );
  }

  return (
    <Screen edges={['top', 'left', 'right']}>
      <View style={{ paddingBottom: space.xl }}>
        <View style={{ height: minTouchTarget, justifyContent: 'center' }}>
          <Pressable
            onPress={goBack}
            accessibilityRole="button"
            accessibilityLabel={`Back to ${PLACE_NAME[from]}`}
            hitSlop={{ right: 16 }}
            style={{ height: minTouchTarget, flexDirection: 'row', alignItems: 'center', alignSelf: 'flex-start' }}
          >
            <Ionicons name="chevron-back" size={24} color={color.ink} />
            <View style={{ marginLeft: space.xs }}>
              <AppText role="body">{PLACE_NAME[from]}</AppText>
            </View>
          </Pressable>
        </View>

        <View style={{ marginTop: space.sm }}>
          <AppText role="title" accessibilityRole="header">
            People
          </AppText>
        </View>

        <View style={{ marginTop: space.lg }}>
          <ListTabs value={tab} onChange={setTab} />
        </View>

        {message !== '' ? (
          <View style={{ marginTop: space.sm }} accessibilityLiveRegion="polite">
            <AppText role="label">{message}</AppText>
          </View>
        ) : null}

        {status === 'loading' ? (
          <View style={{ marginTop: space.lg }}>
            <AppText role="label" tone="muted">
              Loading people.
            </AppText>
          </View>
        ) : status === 'offline' ? (
          <View style={{ marginTop: space.lg }}>
            <StatusMessage kind={failure} onRetry={() => void load()} />
          </View>
        ) : follows !== null ? (
          <>
            {requests.length > 0 ? (
              <View style={{ marginTop: space.lg }}>
                <SectionLabel>Requests</SectionLabel>
                <View style={{ marginTop: space.sm }}>
                  {requests.map((request, index) => {
                    const name = firstName(request.person);
                    const working = busy !== null;
                    return (
                      <PersonRow key={request.request_id} person={request.person} first={index === 0}>
                        <Chip
                          label="Accept"
                          tone="strong"
                          onPress={working ? undefined : () => void act(`accept-${request.request_id}`, () => acceptRequest(request.request_id))}
                          accessibilityLabel={`Accept ${name}`}
                        />
                        <View style={{ marginLeft: space.sm }}>
                          <Chip
                            label="Decline"
                            tone="outline"
                            onPress={working ? undefined : () => void act(`decline-${request.request_id}`, () => declineRequest(request.request_id))}
                            accessibilityLabel={`Decline ${name}`}
                          />
                        </View>
                      </PersonRow>
                    );
                  })}
                </View>
              </View>
            ) : null}

            {tab === 'followers' ? (
              <View style={{ marginTop: requests.length > 0 ? space.xl : space.md }}>
                {requests.length > 0 ? <SectionLabel>Followers</SectionLabel> : null}
                <View style={{ marginTop: requests.length > 0 ? space.sm : 0 }}>
                  {follows.followers.length === 0 ? (
                    <AppText role="label" tone="muted">
                      No followers yet.
                    </AppText>
                  ) : (
                    follows.followers.map((person, index) => (
                      <PersonRow
                        key={person.id}
                        person={person}
                        first={index === 0}
                        onPress={() => setMenu({ person, kind: 'follower' })}
                        pressHint="Opens Remove follower and Block."
                      >
                        {followerButton(person)}
                        <MoreButton name={firstName(person)} onPress={() => setMenu({ person, kind: 'follower' })} />
                      </PersonRow>
                    ))
                  )}
                </View>
              </View>
            ) : (
              <View style={{ marginTop: space.md }}>
                {follows.following.length === 0 && outgoing.length === 0 ? (
                  <AppText role="label" tone="muted">
                    You don't follow anyone yet.
                  </AppText>
                ) : (
                  <>
                    {follows.following.map((person, index) => (
                      <PersonRow
                        key={person.id}
                        person={person}
                        first={index === 0}
                        onPress={() => setMenu({ person, kind: 'following' })}
                        pressHint="Opens Unfollow and Block."
                      >
                        <Chip label="Following" tone="outline" onPress={() => setMenu({ person, kind: 'following' })} />
                      </PersonRow>
                    ))}
                    {outgoing.map((request, index) => (
                      <PersonRow key={request.request_id} person={request.person} first={follows.following.length === 0 && index === 0}>
                        <Chip
                          label="Requested"
                          tone="outline"
                          onPress={() => doUnfollow(request.person)}
                          accessibilityLabel={`Requested. Cancel the request to ${firstName(request.person)}`}
                        />
                      </PersonRow>
                    ))}
                  </>
                )}
              </View>
            )}
          </>
        ) : null}
      </View>
      <MenuSheet visible={menu !== null} items={menuItems} onClose={() => setMenu(null)} />
    </Screen>
  );
}

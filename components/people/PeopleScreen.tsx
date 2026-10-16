import { ReactNode, useCallback, useRef, useState } from 'react';
import { Alert, Pressable, Text, View } from 'react-native';
import { useFocusEffect, useRouter } from 'expo-router';
import { Ionicons } from '@expo/vector-icons';
import { SizedAvatar } from '@/components/profile/SizedAvatar';
import { StatusMessage } from '@/components/today/StatusMessage';
import { AppText } from '@/components/ui/AppText';
import { Chip } from '@/components/ui/Chip';
import { MenuSheet, type MenuItem } from '@/components/ui/MenuSheet';
import { PrimaryButton } from '@/components/ui/PrimaryButton';
import { Screen } from '@/components/ui/Screen';
import { TextField } from '@/components/ui/TextField';
import { blockUser } from '@/lib/api/account';
import { ApiError } from '@/lib/api/client';
import {
  acceptRequest,
  declineRequest,
  firstName,
  follow,
  getFollows,
  lookupPerson,
  removeFollower,
  unfollow,
  type Follows,
  type Lookup,
  type Person,
} from '@/lib/api/social';
import { color, minTouchTarget, space, type } from '@/lib/theme/tokens';

const AVATAR = 40;
const ROW_MIN_HEIGHT = 56;
const USERNAME_MAX = 30;
const GENERIC = 'Something went wrong. Please try again.';

type Place = 'profile' | 'board';
const PLACE_NAME: Record<Place, string> = { profile: 'Profile', board: 'Board' };
const PLACE_ROOT: Record<Place, '/(app)/profile' | '/(app)/board'> = { profile: '/(app)/profile', board: '/(app)/board' };

type Found = { kind: 'idle' } | { kind: 'miss' } | { kind: 'hit'; lookup: Lookup };

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
 * Followers figure on Profile, from the Board's 'Find people' and from a follow notification, with
 * a thin route in each stack. Top to bottom: the username box and 'Find'; the person found with one
 * button ('Follow', 'Follow back', 'Requested' (tap cancels) or 'Following' (tap opens Unfollow and
 * Block)); Requests (Accept, Decline); Followers (a row opens Remove follower and Block); Following
 * (a row opens Unfollow and Block). Nothing here is red; a failure is one quiet line in ink.
 */
export function PeopleScreen({ from }: { from: Place }) {
  const router = useRouter();
  const [follows, setFollows] = useState<Follows | null>(null);
  const [status, setStatus] = useState<'loading' | 'ready' | 'offline'>('loading');
  const [failure, setFailure] = useState<'network' | 'generic'>('network');
  const [query, setQuery] = useState('');
  const [found, setFound] = useState<Found>({ kind: 'idle' });
  const [finding, setFinding] = useState(false);
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

  /** Keeps the person found in step with what was just done to them. */
  function patchFound(personId: string, patch: Partial<Lookup>) {
    setFound((now) =>
      now.kind === 'hit' && now.lookup.person.id === personId ? { kind: 'hit', lookup: { ...now.lookup, ...patch } } : now
    );
  }

  async function find() {
    const username = query.trim().replace(/^@/, '');
    if (username === '' || finding) return;
    setFinding(true);
    setMessage('');
    try {
      const lookup = await lookupPerson(username);
      setFound(lookup === null ? { kind: 'miss' } : { kind: 'hit', lookup });
    } catch (err) {
      setFound({ kind: 'idle' });
      setMessage(messageFor(err));
    } finally {
      setFinding(false);
    }
  }

  function doFollow(person: Person) {
    void act(`follow-${person.id}`, async () => {
      const relation = await follow(person.id);
      patchFound(person.id, { relation });
    });
  }

  function doUnfollow(person: Person) {
    void act(`unfollow-${person.id}`, async () => {
      await unfollow(person.id);
      patchFound(person.id, { relation: 'none' });
    });
  }

  function doRemove(person: Person) {
    void act(`remove-${person.id}`, async () => {
      await removeFollower(person.id);
      patchFound(person.id, { follows_you: false });
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
            setFound((now) => (now.kind === 'hit' && now.lookup.person.id === person.id ? { kind: 'idle' } : now));
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

  /** The one button of the person found. */
  function resultButton(lookup: Lookup): ReactNode {
    const { person, relation } = lookup;
    const name = firstName(person);
    if (relation === 'following') {
      return (
        <Chip
          label="Following"
          tone="outline"
          onPress={() => setMenu({ person, kind: 'following' })}
          accessibilityHint="Opens Unfollow and Block."
        />
      );
    }
    if (relation === 'requested') {
      return (
        <Chip
          label="Requested"
          tone="outline"
          onPress={() => doUnfollow(person)}
          accessibilityLabel={`Requested. Cancel the request to ${name}`}
        />
      );
    }
    return (
      <Chip
        label={lookup.follows_you ? 'Follow back' : 'Follow'}
        tone="strong"
        onPress={() => doFollow(person)}
        accessibilityLabel={`${lookup.follows_you ? 'Follow back' : 'Follow'} ${name}`}
      />
    );
  }

  const followingIds = new Set((follows?.following ?? []).map((person) => person.id));
  const outgoing = (follows?.outgoing ?? []).filter((request) => !followingIds.has(request.person.id));

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

        {/* Find: an exact username, never an email. */}
        <View style={{ marginTop: space.lg }}>
          <TextField
            label="Find by username"
            variant="soft"
            prefix="@"
            value={query}
            onChangeText={(next) => {
              setQuery(next);
              if (found.kind === 'miss') setFound({ kind: 'idle' });
            }}
            autoCapitalize="none"
            autoCorrect={false}
            autoComplete="off"
            maxLength={USERNAME_MAX}
            returnKeyType="search"
            onSubmitEditing={() => void find()}
          />
        </View>
        <View style={{ marginTop: space.md }}>
          <PrimaryButton label="Find" onPress={() => void find()} disabled={query.trim().replace(/^@/, '') === ''} loading={finding} />
        </View>
        {message !== '' ? (
          <View style={{ marginTop: space.sm }} accessibilityLiveRegion="polite">
            <AppText role="label">{message}</AppText>
          </View>
        ) : null}
        <View accessibilityLiveRegion="polite" style={{ marginTop: found.kind === 'idle' ? 0 : space.md }}>
          {found.kind === 'miss' ? (
            <AppText role="label" tone="muted">
              No one has that username.
            </AppText>
          ) : found.kind === 'hit' ? (
            <PersonRow
              first
              person={found.lookup.person}
              onPress={
                found.lookup.relation === 'following'
                  ? () => setMenu({ person: found.lookup.person, kind: 'following' })
                  : undefined
              }
            >
              {resultButton(found.lookup)}
              {found.lookup.relation !== 'following' ? (
                <MoreButton
                  name={firstName(found.lookup.person)}
                  onPress={() => setMenu({ person: found.lookup.person, kind: 'other' })}
                />
              ) : null}
            </PersonRow>
          ) : null}
        </View>

        {status === 'loading' ? (
          <View style={{ marginTop: space.xl }}>
            <AppText role="label" tone="muted">
              Loading people.
            </AppText>
          </View>
        ) : status === 'offline' ? (
          <View style={{ marginTop: space.xl }}>
            <StatusMessage kind={failure} onRetry={() => void load()} />
          </View>
        ) : follows !== null ? (
          <>
            {follows.incoming.length > 0 ? (
              <View style={{ marginTop: space.xl }}>
                <SectionLabel>Requests</SectionLabel>
                <View style={{ marginTop: space.sm }}>
                  {follows.incoming.map((request, index) => {
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

            <View style={{ marginTop: space.xl }}>
              <SectionLabel>Followers</SectionLabel>
              <View style={{ marginTop: space.sm }}>
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
                      <MoreButton name={firstName(person)} onPress={() => setMenu({ person, kind: 'follower' })} />
                    </PersonRow>
                  ))
                )}
              </View>
            </View>

            <View style={{ marginTop: space.xl }}>
              <SectionLabel>Following</SectionLabel>
              <View style={{ marginTop: space.sm }}>
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
            </View>
          </>
        ) : null}
      </View>
      <MenuSheet visible={menu !== null} items={menuItems} onClose={() => setMenu(null)} />
    </Screen>
  );
}

import { useEffect, useRef, useState } from 'react';
import { Alert, Pressable, ScrollView, Share, TextInput, View } from 'react-native';
import { useRouter } from 'expo-router';
import { Ionicons } from '@expo/vector-icons';
import { PersonRow } from '@/components/people/PeopleScreen';
import { AppText } from '@/components/ui/AppText';
import { Chip } from '@/components/ui/Chip';
import { MenuSheet, type MenuItem } from '@/components/ui/MenuSheet';
import { Screen } from '@/components/ui/Screen';
import { blockUser } from '@/lib/api/account';
import { getSuggestions } from '@/lib/api/person';
import { PrimaryButton } from '@/components/ui/PrimaryButton';
import { ApiError, inviteLink } from '@/lib/api/client';
import { firstName, follow, searchPeople, unfollow, type Person, type SearchHit } from '@/lib/api/social';
import { useOpenPerson } from '@/lib/people/openPerson';
import { useSession } from '@/lib/session/store';
import { color, minTouchTarget, radius, space, type } from '@/lib/theme/tokens';

/** The search waits this long after the last key, so a fast typist sends one request, not ten. */
const DEBOUNCE_MS = 250;
const FIELD_HEIGHT = 44;
const QUERY_MAX = 30;
const GENERIC = 'Something went wrong. Please try again.';

function messageFor(err: unknown): string {
  return err instanceof ApiError && (err.code === 'network_unavailable' || err.code === 'rate_limited') ? err.userMessage : GENERIC;
}

/**
 * Search (developer, 2026-10-16): opened from the search icon in the Board's header. One search box:
 * results come as you type, by the start of a username, a name or a word of a name. Each person has
 * one button: 'Follow' or 'Follow back', 'Requested' (tap cancels) or 'Following' (tap opens Unfollow
 * and Block). Nothing here is red; a failure is one quiet line.
 *
 * Find your friends (developer, 2026-10-19): with the box empty the page shows 'Invite friends'
 * (the phone's share sheet with your username) and 'Suggested for you' (people who follow you, then
 * people the people you follow follow). `welcome` is the same page once after sign-up: a title
 * 'Find your friends' in place of Back, no keyboard at first, and 'Done' to go on to the Spin page.
 */
export function SearchScreen({ welcome = false }: { welcome?: boolean }) {
  const router = useRouter();
  const openPerson = useOpenPerson();
  const { user } = useSession();
  const [suggestions, setSuggestions] = useState<SearchHit[] | null>(null);
  const [query, setQuery] = useState('');
  const [hits, setHits] = useState<SearchHit[] | null>(null);
  const [searching, setSearching] = useState(false);
  const [message, setMessage] = useState('');
  const [menu, setMenu] = useState<Person | null>(null);
  const seqRef = useRef(0);
  const busyRef = useRef(false);

  // Ask again a moment after the last key; only the newest answer is used.
  useEffect(() => {
    const value = query.trim().replace(/^@/, '');
    if (value === '') {
      seqRef.current += 1;
      setHits(null);
      setSearching(false);
      return undefined;
    }
    setSearching(true);
    const seq = ++seqRef.current;
    const id = setTimeout(async () => {
      try {
        const next = await searchPeople(value);
        if (seq !== seqRef.current) return;
        setHits(next);
        setMessage('');
      } catch (err) {
        if (seq !== seqRef.current) return;
        setMessage(messageFor(err));
      } finally {
        if (seq === seqRef.current) setSearching(false);
      }
    }, DEBOUNCE_MS);
    return () => clearTimeout(id);
  }, [query]);

  // The suggestions, read once when the page opens; a failure leaves the list off.
  useEffect(() => {
    let alive = true;
    getSuggestions()
      .then((next) => {
        if (alive) setSuggestions(next);
      })
      .catch(() => {
        if (alive) setSuggestions([]);
      });
    return () => {
      alive = false;
    };
  }, []);

  function goBack() {
    if (router.canGoBack()) router.back();
    else router.replace('/(app)/board');
  }

  function done() {
    router.replace('/(app)/today');
  }

  /** The phone's share sheet with your invite link (it opens the app at your profile). */
  function invite() {
    const username = user?.username ?? '';
    const message =
      username !== ''
        ? `Join me on RNDMRoll, one spin a day. Follow me here: ${inviteLink(username)}`
        : 'Join me on RNDMRoll, one spin a day.';
    void Share.share({ message }).catch(() => undefined);
  }

  function patch(personId: string, next: Partial<SearchHit>) {
    const apply = (list: SearchHit[] | null) =>
      list === null ? list : list.map((hit) => (hit.person.id === personId ? { ...hit, ...next } : hit));
    setHits(apply);
    setSuggestions(apply);
  }

  async function act(work: () => Promise<void>) {
    if (busyRef.current) return;
    busyRef.current = true;
    setMessage('');
    try {
      await work();
    } catch (err) {
      setMessage(messageFor(err));
    } finally {
      busyRef.current = false;
    }
  }

  function doFollow(person: Person) {
    void act(async () => patch(person.id, { relation: await follow(person.id) }));
  }

  function doUnfollow(person: Person) {
    void act(async () => {
      await unfollow(person.id);
      patch(person.id, { relation: 'none' });
    });
  }

  function askBlock(person: Person) {
    const name = firstName(person);
    Alert.alert(`Block ${name}?`, "You will stop seeing each other's posts. They won't be told.", [
      { text: "Don't block", style: 'cancel' },
      {
        text: 'Block',
        onPress: () =>
          void act(async () => {
            await blockUser(person.id);
            setHits((now) => (now === null ? now : now.filter((hit) => hit.person.id !== person.id)));
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
      : [
          { label: 'Unfollow', onPress: fromMenu(() => doUnfollow(menu)) },
          { label: 'Block', onPress: fromMenu(() => askBlock(menu)) },
        ];

  function button(hit: SearchHit) {
    const { person, relation } = hit;
    const name = firstName(person);
    if (relation === 'following') {
      return <Chip label="Following" tone="outline" onPress={() => setMenu(person)} accessibilityHint="Opens Unfollow and Block." />;
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
    const label = hit.follows_you ? 'Follow back' : 'Follow';
    return <Chip label={label} tone="strong" onPress={() => doFollow(person)} accessibilityLabel={`${label} ${name}`} />;
  }

  const trimmed = query.trim().replace(/^@/, '');

  return (
    <Screen edges={['top', 'left', 'right']} scroll={false}>
      <View style={{ flex: 1 }}>
        {welcome ? (
          <View style={{ marginTop: space.md, marginBottom: space.md }}>
            <AppText role="title" accessibilityRole="header">
              Find your friends
            </AppText>
            <View style={{ marginTop: space.xs }}>
              <AppText role="body" tone="muted">
                Follow people you know to see their spins on your Board.
              </AppText>
            </View>
          </View>
        ) : null}
        {/* Back, then the one search box. */}
        <View style={{ height: minTouchTarget, flexDirection: 'row', alignItems: 'center' }}>
          {welcome ? null : (
            <Pressable
              onPress={goBack}
              accessibilityRole="button"
              accessibilityLabel="Back to Board"
              hitSlop={{ right: 8 }}
              style={{ height: minTouchTarget, justifyContent: 'center', marginRight: space.sm }}
            >
              <Ionicons name="chevron-back" size={24} color={color.ink} />
            </Pressable>
          )}
          <View
            style={{
              flex: 1,
              height: FIELD_HEIGHT,
              flexDirection: 'row',
              alignItems: 'center',
              paddingHorizontal: space.md,
              borderRadius: radius.md,
              backgroundColor: color.secondary,
            }}
          >
            <Ionicons name="search-outline" size={18} color={color.muted} />
            <TextInput
              value={query}
              onChangeText={setQuery}
              placeholder="Search"
              placeholderTextColor={color.muted}
              // No keyboard at first, so Invite friends and the suggestions are in view.
              autoFocus={false}
              autoCapitalize="none"
              autoCorrect={false}
              autoComplete="off"
              returnKeyType="search"
              maxLength={QUERY_MAX}
              accessibilityLabel="Search people by name or username"
              selectionColor={color.ink}
              style={{
                flex: 1,
                marginLeft: space.sm,
                height: FIELD_HEIGHT,
                fontFamily: type.body.fontFamily,
                fontSize: type.body.fontSize,
                color: color.ink,
              }}
            />
            {query !== '' ? (
              <Pressable onPress={() => setQuery('')} accessibilityRole="button" accessibilityLabel="Clear" hitSlop={10}>
                <Ionicons name="close-circle" size={18} color={color.muted} />
              </Pressable>
            ) : null}
          </View>
        </View>

        {message !== '' ? (
          <View style={{ marginTop: space.sm }} accessibilityLiveRegion="polite">
            <AppText role="label">{message}</AppText>
          </View>
        ) : null}

        <ScrollView style={{ flex: 1, marginTop: space.md }} keyboardShouldPersistTaps="handled" keyboardDismissMode="on-drag">
          {trimmed === '' ? (
            <View>
              {/* Invite friends: your link, sent with the phone's share sheet. */}
              <View style={{ paddingVertical: space.sm }}>
                <AppText role="button">Invite friends</AppText>
                <View style={{ marginTop: space.xs }}>
                  <AppText role="body" tone="muted">
                    Send your link. It opens RNDMRoll at your profile so friends can follow you.
                  </AppText>
                </View>
                <View style={{ marginTop: space.md }}>
                  <PrimaryButton label="Send invite link" onPress={invite} />
                </View>
              </View>
              {suggestions !== null && suggestions.length > 0 ? (
                <View style={{ marginTop: space.xl }}>
                  <AppText role="button">Suggested for you</AppText>
                  <View style={{ marginTop: space.sm }}>
                    {suggestions.map((hit, index) => (
                      <PersonRow
                        key={hit.person.id}
                        person={hit.person}
                        first={index === 0}
                        onPress={() => openPerson(hit.person.id)}
                        pressHint="Opens their profile."
                      >
                        {button(hit)}
                      </PersonRow>
                    ))}
                  </View>
                </View>
              ) : null}
            </View>
          ) : hits === null ? (
            <AppText role="label" tone="muted">
              {searching ? 'Searching.' : ''}
            </AppText>
          ) : hits.length === 0 ? (
            <AppText role="label" tone="muted" accessibilityLiveRegion="polite">
              {searching ? 'Searching.' : 'No one found.'}
            </AppText>
          ) : (
            hits.map((hit, index) => (
              <PersonRow
                key={hit.person.id}
                person={hit.person}
                first={index === 0}
                onPress={() => openPerson(hit.person.id)}
                pressHint="Opens their profile."
              >
                {button(hit)}
              </PersonRow>
            ))
          )}
          <View style={{ height: space.xl }} />
        </ScrollView>
        {welcome ? (
          <View style={{ paddingVertical: space.md }}>
            <PrimaryButton label="Done" onPress={done} />
          </View>
        ) : null}
      </View>
      <MenuSheet visible={menu !== null} items={menuItems} onClose={() => setMenu(null)} />
    </Screen>
  );
}

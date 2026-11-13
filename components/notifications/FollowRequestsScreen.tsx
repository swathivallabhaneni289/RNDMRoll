import { useCallback, useRef, useState } from 'react';
import { Pressable, ScrollView, View } from 'react-native';
import { useFocusEffect, useRouter } from 'expo-router';
import { Ionicons } from '@expo/vector-icons';
import { SafeAreaView } from 'react-native-safe-area-context';
import { PersonRow } from '@/components/people/PeopleScreen';
import { StatusMessage } from '@/components/today/StatusMessage';
import { AppText } from '@/components/ui/AppText';
import { Chip } from '@/components/ui/Chip';
import { ApiError } from '@/lib/api/client';
import {
  answerRequest,
  firstName,
  follow,
  getFollows,
  type FollowRequest,
  type Follows,
  type Person,
  type Relation,
} from '@/lib/api/social';
import { useForegroundReload } from '@/lib/api/useForegroundReload';
import { useOpenPerson } from '@/lib/people/openPerson';
import { color, minTouchTarget, space } from '@/lib/theme/tokens';

const GENERIC = 'Something went wrong. Please try again.';

/** A request that was accepted on this page: the row stays with "Follow back" until the person leaves. */
type Accepted = { person: Person; relation: Relation };

/**
 * Follow requests, like Instagram's: opened from the "Follow requests" row at the top of the bell
 * list. Every follow is a request (developer, 2026-10-16), so this is where they are answered.
 * Each row has Accept and Decline. An accepted row stays with "Follow back" (or "Following" or
 * "Requested" when that is already true) until the person leaves; a declined row goes at once,
 * and the other person is never told. A request that went away while the page was open (cancelled,
 * or a block) leaves at the first tap on either button, with no line: cancels are silent (SOC-02).
 * One whose earlier Accept went through with its answer lost shows as accepted. A failed tap keeps
 * the row for another try. No bottom bar, so the page keeps all four safe-area edges.
 */
export function FollowRequestsScreen() {
  const router = useRouter();
  const openPerson = useOpenPerson();
  const [requests, setRequests] = useState<FollowRequest[] | null>(null);
  const [accepted, setAccepted] = useState<Record<string, Accepted>>({});
  const [status, setStatus] = useState<'loading' | 'ready' | 'offline'>('loading');
  const [failure, setFailure] = useState<'network' | 'generic'>('network');
  const [message, setMessage] = useState('');
  const [busy, setBusy] = useState(false);
  const busyRef = useRef(false);
  const loadedRef = useRef(false);

  // Read again on a return from the background once the list is 20 minutes old (photo links run out).
  const { markLoaded } = useForegroundReload(() => void load());

  const load = useCallback(async () => {
    try {
      const next = await getFollows();
      setRequests(next.incoming);
      loadedRef.current = true;
      markLoaded();
      setStatus('ready');
    } catch (err) {
      if (loadedRef.current) return;
      setFailure(err instanceof ApiError && err.code === 'network_unavailable' ? 'network' : 'generic');
      setStatus('offline');
    }
  }, [markLoaded]);

  useFocusEffect(
    useCallback(() => {
      void load();
    }, [load]),
  );

  function goBack() {
    if (router.canGoBack()) router.back();
    else router.replace('/(app)/today/notifications');
  }

  /** Runs one action at a time; a failure shows one plain line. */
  async function act(work: () => Promise<void>) {
    if (busyRef.current) return;
    busyRef.current = true;
    setBusy(true);
    setMessage('');
    try {
      await work();
    } catch (err) {
      setMessage(err instanceof ApiError && err.code === 'network_unavailable' ? err.userMessage : GENERIC);
    } finally {
      busyRef.current = false;
      setBusy(false);
    }
  }

  /** The row as accepted. What the caller already is toward this person decides the button left on it. */
  function showAccepted(request: FollowRequest, lists: Follows | null) {
    const relation: Relation = lists?.following.some((p) => p.id === request.person.id)
      ? 'following'
      : lists?.outgoing.some((r) => r.person.id === request.person.id)
        ? 'requested'
        : 'none';
    setAccepted((now) => ({
      ...now,
      [request.request_id]: { person: request.person, relation },
    }));
    if (lists) setRequests(lists.incoming);
  }

  function accept(request: FollowRequest) {
    void act(async () => {
      const answer = await answerRequest(request.request_id, 'accept');
      const lists = await getFollows().catch(() => null);
      if (answer === 'gone' && !lists?.followers.some((p) => p.id === request.person.id)) {
        // Cancelled meanwhile, or a block: the row goes without a word. Gone while they follow you
        // means an earlier Accept went through with its answer lost, and the row shows as accepted.
        setRequests((now) => (lists ? lists.incoming : (now ?? []).filter((r) => r.request_id !== request.request_id)));
        return;
      }
      showAccepted(request, lists);
    });
  }

  function decline(request: FollowRequest) {
    void act(async () => {
      const answer = await answerRequest(request.request_id, 'decline');
      if (answer === 'gone') {
        // Cancelled meanwhile, or a block: the row goes as on a decline. Unless an earlier Accept
        // went through with its answer lost: they follow you now, and the row shows as accepted.
        const lists = await getFollows().catch(() => null);
        if (lists?.followers.some((p) => p.id === request.person.id)) {
          showAccepted(request, lists);
          return;
        }
      }
      setRequests((now) => (now ?? []).filter((r) => r.request_id !== request.request_id));
    });
  }

  function followBack(requestId: string, person: Person) {
    void act(async () => {
      const relation = await follow(person.id);
      setAccepted((now) => ({ ...now, [requestId]: { person, relation } }));
    });
  }

  const acceptedRows = Object.entries(accepted);
  const pending = (requests ?? []).filter((r) => accepted[r.request_id] === undefined);
  const empty = pending.length === 0 && acceptedRows.length === 0;

  return (
    <SafeAreaView style={{ flex: 1, backgroundColor: color.dominant }}>
      <ScrollView
        contentContainerStyle={{
          paddingHorizontal: space.lg,
          paddingBottom: space.xl,
        }}
      >
        <View style={{ paddingTop: space.sm }}>
          <Pressable
            onPress={goBack}
            accessibilityRole="button"
            accessibilityLabel="Back"
            hitSlop={8}
            style={{
              width: minTouchTarget,
              height: minTouchTarget,
              justifyContent: 'center',
              alignItems: 'flex-start',
            }}
          >
            <Ionicons name="chevron-back" size={28} color={color.ink} />
          </Pressable>
        </View>

        <View style={{ marginTop: space.md }}>
          <AppText role="title" accessibilityRole="header">
            Follow requests
          </AppText>
        </View>

        {message !== '' ? (
          <View style={{ marginTop: space.sm }} accessibilityLiveRegion="polite">
            <AppText role="label">{message}</AppText>
          </View>
        ) : null}

        <View style={{ marginTop: space.lg }}>
          {status === 'loading' ? (
            <AppText role="label" tone="muted">
              Loading your requests.
            </AppText>
          ) : status === 'offline' ? (
            <StatusMessage kind={failure} onRetry={() => void load()} />
          ) : empty ? (
            <AppText role="label" tone="muted">
              No requests right now.
            </AppText>
          ) : (
            <>
              {pending.map((request, index) => {
                const name = firstName(request.person);
                return (
                  <PersonRow
                    key={request.request_id}
                    person={request.person}
                    first={index === 0}
                    onPress={() => openPerson(request.person.id)}
                    pressHint="Opens their profile."
                  >
                    <Chip
                      label="Accept"
                      tone="strong"
                      onPress={busy ? undefined : () => accept(request)}
                      accessibilityLabel={`Accept ${name}`}
                    />
                    <View style={{ marginLeft: space.sm }}>
                      <Chip
                        label="Decline"
                        tone="outline"
                        onPress={busy ? undefined : () => decline(request)}
                        accessibilityLabel={`Decline ${name}`}
                      />
                    </View>
                  </PersonRow>
                );
              })}
              {acceptedRows.map(([requestId, row], index) => {
                const name = firstName(row.person);
                return (
                  <PersonRow
                    key={requestId}
                    person={row.person}
                    first={pending.length === 0 && index === 0}
                    onPress={() => openPerson(row.person.id)}
                    pressHint="Opens their profile."
                  >
                    {row.relation === 'none' ? (
                      <Chip
                        label="Follow back"
                        tone="strong"
                        onPress={busy ? undefined : () => followBack(requestId, row.person)}
                        accessibilityLabel={`Follow back ${name}`}
                      />
                    ) : (
                      <Chip
                        label={row.relation === 'following' ? 'Following' : 'Requested'}
                        tone="outline"
                        accessibilityLabel={`${row.relation === 'following' ? 'Following' : 'Requested'} ${name}`}
                      />
                    )}
                  </PersonRow>
                );
              })}
            </>
          )}
        </View>
      </ScrollView>
    </SafeAreaView>
  );
}

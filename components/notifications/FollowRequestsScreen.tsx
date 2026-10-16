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
  acceptRequest,
  declineRequest,
  firstName,
  follow,
  getFollows,
  type FollowRequest,
  type Person,
  type Relation,
} from '@/lib/api/social';
import { color, minTouchTarget, space } from '@/lib/theme/tokens';

const GENERIC = 'Something went wrong. Please try again.';

/** A request that was accepted on this page: the row stays with "Follow back" until the person leaves. */
type Accepted = { person: Person; relation: Relation };

/**
 * Follow requests, like Instagram's: opened from the "Follow requests" row at the top of the bell
 * list. Every follow is a request (developer, 2026-10-16), so this is where they are answered.
 * Each row has Accept and Decline. An accepted row stays with "Follow back" (or "Following" or
 * "Requested" when that is already true) until the person leaves; a declined row goes at once,
 * and the other person is never told. No bottom bar, so the page keeps all four safe-area edges.
 */
export function FollowRequestsScreen({ place = 'today' }: { place?: 'today' | 'board' }) {
  const router = useRouter();
  const [requests, setRequests] = useState<FollowRequest[] | null>(null);
  const [accepted, setAccepted] = useState<Record<string, Accepted>>({});
  const [status, setStatus] = useState<'loading' | 'ready' | 'offline'>('loading');
  const [failure, setFailure] = useState<'network' | 'generic'>('network');
  const [message, setMessage] = useState('');
  const [busy, setBusy] = useState(false);
  const busyRef = useRef(false);
  const loadedRef = useRef(false);

  const load = useCallback(async () => {
    try {
      const next = await getFollows();
      setRequests(next.incoming);
      loadedRef.current = true;
      setStatus('ready');
    } catch (err) {
      if (loadedRef.current) return;
      setFailure(err instanceof ApiError && err.code === 'network_unavailable' ? 'network' : 'generic');
      setStatus('offline');
    }
  }, []);

  useFocusEffect(
    useCallback(() => {
      void load();
    }, [load]),
  );

  function goBack() {
    if (router.canGoBack()) router.back();
    else router.replace(place === 'board' ? '/(app)/board/notifications' : '/(app)/today/notifications');
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

  function accept(request: FollowRequest) {
    void act(async () => {
      await acceptRequest(request.request_id);
      // What the caller already is toward this person decides the button left on the row.
      const lists = await getFollows().catch(() => null);
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
    });
  }

  function decline(request: FollowRequest) {
    void act(async () => {
      await declineRequest(request.request_id);
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
                  <PersonRow key={request.request_id} person={request.person} first={index === 0}>
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
                  <PersonRow key={requestId} person={row.person} first={pending.length === 0 && index === 0}>
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

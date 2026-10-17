import { useCallback, useRef, useState } from 'react';
import { Pressable, ScrollView, Text, View } from 'react-native';
import { useFocusEffect, useRouter } from 'expo-router';
import { Image } from 'expo-image';
import { Ionicons } from '@expo/vector-icons';
import { SafeAreaView } from 'react-native-safe-area-context';
import { Circle, Svg } from 'react-native-svg';
import { BrandMark } from '@/components/brand/BrandMark';
import { UnreadDot } from '@/components/notifications/BellButton';
import { SizedAvatar } from '@/components/profile/SizedAvatar';
import { StatusMessage } from '@/components/today/StatusMessage';
import { AppText } from '@/components/ui/AppText';
import { Chip } from '@/components/ui/Chip';
import { getPost } from '@/lib/api/board';
import { ApiError } from '@/lib/api/client';
import {
  getNotifications,
  markNotificationsSeen,
  type NotificationItem,
  type NotificationSection,
} from '@/lib/api/notifications';
import { firstName, follow, getFollows, type Person, type Relation } from '@/lib/api/social';
import { REACTIONS } from '@/lib/entry/reactions';
import { setUnread } from '@/lib/notifications/unreadStore';
import { useSession } from '@/lib/session/store';
import { color, minTouchTarget, radius, space, type } from '@/lib/theme/tokens';
import { useOpenPerson } from '@/lib/people/openPerson';

const AVATAR = 44;
const THUMB = 44;
const MARK = 20;
/** The gap between the avatar, the words and what sits at the right. */
const GAP = space.sm + space.xs;

const SECTIONS: { key: NotificationSection; title: string }[] = [
  { key: 'today', title: 'Today' },
  { key: 'week', title: 'This week' },
  { key: 'earlier', title: 'Earlier' },
];

/** A row's words: the Label size in ink, so a row reads like Instagram's and two or three fit a line. */
const rowText = {
  fontFamily: type.label.fontFamily,
  fontSize: type.label.fontSize,
  lineHeight: type.label.lineHeight,
  color: color.ink,
} as const;

/** The sentence of a row, split so the person's first name can be drawn in the button weight. */
function sentence(item: NotificationItem): { name: string; rest: string; spoken: string } {
  const name = item.actor ? firstName(item.actor) : '';
  const category = item.category_name ? item.category_name.toLowerCase() : 'post';
  const reaction = REACTIONS.find((r) => r.name === item.reaction) ?? null;
  switch (item.kind) {
    case 'reaction':
      return {
        name,
        rest: reaction ? ` reacted ${reaction.char} to your ${category}.` : ` reacted to your ${category}.`,
        spoken: reaction ? `${name} reacted ${reaction.label} to your ${category}.` : `${name} reacted to your ${category}.`,
      };
    case 'comment': {
      const rest = item.text ? ` commented: "${item.text}"` : ` commented on your ${category}.`;
      return { name, rest, spoken: `${name}${rest}` };
    }
    case 'reply': {
      const rest = item.text ? ` replied to your comment: "${item.text}"` : ' replied to your comment.';
      return { name, rest, spoken: `${name}${rest}` };
    }
    case 'follow':
      return { name, rest: ' started following you.', spoken: `${name} started following you.` };
    case 'follow_request':
      return { name, rest: ' wants to follow you.', spoken: `${name} wants to follow you.` };
    case 'follow_accepted':
      return { name, rest: ' accepted your follow request.', spoken: `${name} accepted your follow request.` };
    case 'spin_open':
      return { name: '', rest: 'Your spin is open.', spoken: 'Your spin is open.' };
    case 'bonus_spin':
      return {
        name: '',
        rest: 'You hit your goal this week. One bonus spin added.',
        spoken: 'You hit your goal this week. One bonus spin added.',
      };
  }
}

/** The round mark at the left of a row: the person, the wheel mark, or a solid dot for a bonus. */
function RowMark({ item }: { item: NotificationItem }) {
  if (item.kind === 'spin_open') {
    return (
      <View style={{ width: AVATAR, height: AVATAR, alignItems: 'center', justifyContent: 'center' }}>
        <Svg width={AVATAR} height={AVATAR} style={{ position: 'absolute' }}>
          <Circle cx={AVATAR / 2} cy={AVATAR / 2} r={AVATAR / 2 - 0.75} fill="none" stroke={color.ink} strokeWidth={1.5} />
        </Svg>
        <BrandMark size={MARK} color={color.ink} wedgeColor={color.dominant} />
      </View>
    );
  }
  if (item.kind === 'bonus_spin') {
    return (
      <Svg width={AVATAR} height={AVATAR}>
        <Circle cx={AVATAR / 2} cy={AVATAR / 2} r={AVATAR / 2} fill={color.ink} />
        <Circle cx={AVATAR / 2} cy={AVATAR / 2} r={4} fill={color.dominant} />
      </Svg>
    );
  }
  const person = item.actor;
  return (
    <SizedAvatar
      size={AVATAR}
      name={person ? person.name || person.username : ''}
      uri={person?.photo_url ?? null}
      placeholder="initials"
    />
  );
}

/** The post's first photo, small and square, at the right of a reaction, comment or reply. */
function Thumb({ uri }: { uri: string }) {
  const [failed, setFailed] = useState(false);
  return (
    <View style={{ width: THUMB, height: THUMB, borderRadius: radius.sm, overflow: 'hidden', backgroundColor: color.secondary }}>
      {!failed ? (
        <Image source={{ uri }} style={{ width: THUMB, height: THUMB }} contentFit="cover" transition={0} onError={() => setFailed(true)} />
      ) : null}
    </View>
  );
}

/**
 * The page behind the bell (05-UI-SPEC section 10), on the Spin page's stack only (the Board's bell
 * was removed 2026-10-16), redrawn like Instagram's activity page (developer, 2026-10-17): a slim
 * header with Back and the name, the Follow requests row (a dot when one waits, no name or count),
 * then Today, This week and Earlier. Each row is the person's photo, one short sentence with the
 * time after it in grey, and at the right the post's picture (a reaction, comment or reply) or a
 * Follow back, Requested or Following button (a new follower). No lines between rows; an unread
 * row has a small dot in the left margin. The server builds the list, its sections and every time
 * label. Opening the page marks everything seen, so the bell's dot goes out; the dots it loaded
 * with stay until the person leaves.
 *
 * A tap opens your own entry page (a reaction or comment), the post a reply is on (your entry page
 * when it is yours), People on Followers (a new follower) or Following (an accepted request), or
 * the Spin page (your spin is open, a bonus spin). No bottom bar, so the page keeps all four
 * safe-area edges.
 */
export function NotificationsScreen() {
  const router = useRouter();
  const openPerson = useOpenPerson();
  const { user } = useSession();
  const userId = user?.id ?? null;
  const [items, setItems] = useState<NotificationItem[] | null>(null);
  const [relations, setRelations] = useState<Record<string, Relation>>({});
  const [status, setStatus] = useState<'loading' | 'ready' | 'offline'>('loading');
  const [failure, setFailure] = useState<'network' | 'generic'>('network');
  const itemsRef = useRef<NotificationItem[] | null>(null);
  const openingRef = useRef(false);
  const busyRef = useRef(false);

  /** What the viewer is toward each person, for the buttons on follow rows. A failure leaves them off. */
  const loadRelations = useCallback(async () => {
    try {
      const lists = await getFollows();
      const next: Record<string, Relation> = {};
      for (const request of lists.outgoing) next[request.person.id] = 'requested';
      for (const person of lists.following) next[person.id] = 'following';
      setRelations(next);
    } catch {
      // The rows still show; only their buttons wait for the next open.
    }
  }, []);

  const load = useCallback(async () => {
    if (userId === null) return;
    void loadRelations();
    try {
      const answer = await getNotifications();
      // The dots the page opened with stay while it is open, even after the seen call below.
      const shown = new Set((itemsRef.current ?? []).filter((item) => item.unread).map((item) => item.id));
      const next = answer.items.map((item) => (shown.has(item.id) ? { ...item, unread: true } : item));
      itemsRef.current = next;
      setItems(next);
      setStatus('ready');
      try {
        await markNotificationsSeen();
        setUnread(userId, false);
      } catch {
        // Nothing is shown; the next open tries again.
      }
    } catch (err) {
      if (itemsRef.current !== null) return;
      setFailure(err instanceof ApiError && err.code === 'network_unavailable' ? 'network' : 'generic');
      setStatus('offline');
    }
  }, [userId, loadRelations]);

  useFocusEffect(
    useCallback(() => {
      void load();
    }, [load])
  );

  function goBack() {
    if (router.canGoBack()) router.back();
    else router.replace('/(app)/today');
  }

  /** Your own entry page, in this stack, with Back reading 'Notifications'. */
  function openEntry(entryId: string) {
    router.push({
      pathname: '/(app)/today/entry',
      params: { id: entryId, via: 'notifications' },
    });
  }

  /** Someone else's post, in this stack, with Back reading 'Notifications'. */
  function openPost(entryId: string) {
    router.push({
      pathname: '/(app)/today/post',
      params: { id: entryId, via: 'notifications' },
    });
  }

  /**
   * A reply can be on your own entry or on someone else's post, and the item does not say which:
   * one read of the post decides (your own opens your entry page). If the read fails, the post
   * page opens and shows its own gone or offline state.
   */
  async function openReply(entryId: string) {
    if (openingRef.current) return;
    openingRef.current = true;
    let mine = false;
    try {
      mine = (await getPost(entryId)).card.mine;
    } catch {
      mine = false;
    } finally {
      openingRef.current = false;
    }
    if (mine) openEntry(entryId);
    else openPost(entryId);
  }

  function open(item: NotificationItem) {
    if (item.kind === 'reaction' || item.kind === 'comment') {
      if (item.entry_id === null) return;
      openEntry(item.entry_id);
      return;
    }
    if (item.kind === 'reply') {
      if (item.entry_id === null) return;
      void openReply(item.entry_id);
      return;
    }
    if (item.kind === 'spin_open' || item.kind === 'bonus_spin') {
      router.navigate('/(app)/today');
      return;
    }
    // A follow, an accepted request: their profile, like Instagram.
    if (item.actor) openPerson(item.actor.id);
  }

  /** Follow back from the row; every follow is a request, so it turns to Requested. */
  async function followBack(person: Person) {
    if (busyRef.current) return;
    busyRef.current = true;
    try {
      const relation = await follow(person.id);
      setRelations((now) => ({ ...now, [person.id]: relation }));
    } catch {
      // The button stays as it was; a second tap tries again.
    } finally {
      busyRef.current = false;
    }
  }

  /** What sits at the right of a row: the post's picture, a follow button, or nothing. */
  function rowEnd(item: NotificationItem) {
    if ((item.kind === 'reaction' || item.kind === 'comment' || item.kind === 'reply') && item.photo_url !== '') {
      return (
        <View accessibilityElementsHidden importantForAccessibility="no-hide-descendants">
          <Thumb uri={item.photo_url} />
        </View>
      );
    }
    if (item.kind === 'follow' && item.actor) {
      const person = item.actor;
      const name = firstName(person);
      const relation = relations[person.id] ?? 'none';
      if (relation === 'following') return <Chip label="Following" tone="outline" accessibilityLabel={`Following ${name}`} />;
      if (relation === 'requested') return <Chip label="Requested" tone="outline" accessibilityLabel={`Requested ${name}`} />;
      return <Chip label="Follow back" tone="strong" onPress={() => void followBack(person)} accessibilityLabel={`Follow back ${name}`} />;
    }
    return null;
  }

  function renderRow(item: NotificationItem) {
    const words = sentence(item);
    const spoken = `${words.spoken} ${item.at_label}${item.unread ? '. New' : ''}`;
    const end = rowEnd(item);
    return (
      <View key={item.id} style={{ flexDirection: 'row', alignItems: 'center', paddingVertical: space.sm }}>
        {item.unread ? (
          <View style={{ position: 'absolute', left: -space.md, top: 0, bottom: 0, justifyContent: 'center' }}>
            <UnreadDot />
          </View>
        ) : null}
        <Pressable
          onPress={() => open(item)}
          accessibilityRole="button"
          accessibilityLabel={spoken}
          style={{ flex: 1, flexDirection: 'row', alignItems: 'center', minHeight: AVATAR }}
        >
          {item.actor ? (
            <Pressable
              onPress={() => item.actor && openPerson(item.actor.id)}
              accessibilityRole="button"
              accessibilityLabel={`${firstName(item.actor)}'s profile`}
              hitSlop={6}
            >
              <RowMark item={item} />
            </Pressable>
          ) : (
            <View accessibilityElementsHidden importantForAccessibility="no-hide-descendants">
              <RowMark item={item} />
            </View>
          )}
          <View style={{ flex: 1, marginLeft: GAP }}>
            <Text numberOfLines={3} maxFontSizeMultiplier={1.3} style={rowText}>
              {words.name !== '' ? <Text style={{ fontFamily: type.button.fontFamily }}>{words.name}</Text> : null}
              {words.rest}
              {item.at_label !== '' ? <Text style={{ color: color.muted }}>{`  ${item.at_label.replace(/ /g, ' ')}`}</Text> : null}
            </Text>
          </View>
        </Pressable>
        {end ? <View style={{ marginLeft: GAP }}>{end}</View> : null}
      </View>
    );
  }

  // Requests are answered on their own page; the list shows one row for all of them at the top.
  const requests = (items ?? []).filter((item) => item.kind === 'follow_request' && item.actor !== null);
  const sections = SECTIONS.map((section) => ({
    ...section,
    rows: (items ?? []).filter((item) => item.section === section.key && item.kind !== 'follow_request'),
  })).filter((section) => section.rows.length > 0);

  /** Only a dot says there is a request (developer, 2026-10-17): no name and no count. */
  function requestsRow() {
    const waiting = requests.length > 0;
    return (
      <Pressable
        onPress={() => router.push('/(app)/today/follow-requests')}
        accessibilityRole="button"
        accessibilityLabel={waiting ? 'Follow requests. New' : 'Follow requests'}
        accessibilityHint="Opens the page to accept or decline."
        style={{ flexDirection: 'row', alignItems: 'center', paddingVertical: space.sm }}
      >
        <View
          accessibilityElementsHidden
          importantForAccessibility="no-hide-descendants"
          style={{
            width: AVATAR,
            height: AVATAR,
            borderRadius: AVATAR / 2,
            borderWidth: 1,
            borderColor: color.divider,
            alignItems: 'center',
            justifyContent: 'center',
          }}
        >
          <Ionicons name="person-add-outline" size={AVATAR / 2 - 2} color={color.ink} />
        </View>
        <View style={{ flex: 1, marginLeft: GAP }}>
          <Text maxFontSizeMultiplier={1.3} style={{ ...rowText, fontFamily: type.button.fontFamily }}>
            Follow requests
          </Text>
        </View>
        <View style={{ flexDirection: 'row', alignItems: 'center' }}>
          {waiting ? <UnreadDot /> : null}
          <Ionicons name="chevron-forward" size={20} color={color.muted} style={{ marginLeft: space.sm }} />
        </View>
      </Pressable>
    );
  }

  const sectionLabel = {
    fontFamily: type.button.fontFamily,
    fontSize: type.body.fontSize,
    lineHeight: type.body.lineHeight,
    color: color.ink,
  } as const;

  return (
    <SafeAreaView style={{ flex: 1, backgroundColor: color.dominant }}>
      {/* A slim header: Back and the page's name on one line. */}
      <View style={{ height: minTouchTarget + space.sm, flexDirection: 'row', alignItems: 'center', paddingHorizontal: space.lg }}>
        <Pressable
          onPress={goBack}
          accessibilityRole="button"
          accessibilityLabel="Back"
          hitSlop={8}
          style={{ width: minTouchTarget, height: minTouchTarget, justifyContent: 'center', alignItems: 'flex-start', marginLeft: -space.xs }}
        >
          <Ionicons name="chevron-back" size={28} color={color.ink} />
        </Pressable>
        <AppText role="heading" accessibilityRole="header">
          Notifications
        </AppText>
      </View>

      <ScrollView contentContainerStyle={{ paddingHorizontal: space.lg, paddingTop: space.sm, paddingBottom: space.xl }}>
        {status === 'loading' ? (
          <AppText role="label" tone="muted">
            Loading your notifications.
          </AppText>
        ) : status === 'offline' ? (
          <StatusMessage kind={failure} onRetry={() => void load()} />
        ) : (
          <>
            {requestsRow()}
            <View style={{ height: 1, backgroundColor: color.divider, marginTop: space.sm }} />
            {sections.length === 0 ? (
              <View style={{ marginTop: space.lg }}>
                <AppText role="label" tone="muted">
                  Nothing new yet.
                </AppText>
              </View>
            ) : null}
            {sections.map((section) => (
              <View key={section.key} style={{ marginTop: space.lg }}>
                <Text accessibilityRole="header" maxFontSizeMultiplier={1.3} style={sectionLabel}>
                  {section.title}
                </Text>
                <View style={{ marginTop: space.xs }}>{section.rows.map(renderRow)}</View>
              </View>
            ))}
          </>
        )}
      </ScrollView>
    </SafeAreaView>
  );
}

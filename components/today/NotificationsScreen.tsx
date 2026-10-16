import { useCallback, useRef, useState } from 'react';
import { Pressable, ScrollView, Text, View } from 'react-native';
import { useFocusEffect, useRouter } from 'expo-router';
import { Ionicons } from '@expo/vector-icons';
import { SafeAreaView } from 'react-native-safe-area-context';
import { Circle, Svg } from 'react-native-svg';
import { BrandMark } from '@/components/brand/BrandMark';
import { UnreadDot } from '@/components/notifications/BellButton';
import { SizedAvatar } from '@/components/profile/SizedAvatar';
import { StatusMessage } from '@/components/today/StatusMessage';
import { AppText } from '@/components/ui/AppText';
import { Chip } from '@/components/ui/Chip';
import { ApiError } from '@/lib/api/client';
import {
  getNotifications,
  markNotificationsSeen,
  type NotificationItem,
  type NotificationSection,
} from '@/lib/api/notifications';
import { acceptRequest, declineRequest, firstName } from '@/lib/api/social';
import { REACTIONS } from '@/lib/entry/reactions';
import { setUnread } from '@/lib/notifications/unreadStore';
import { useSession } from '@/lib/session/store';
import { color, minTouchTarget, space, type } from '@/lib/theme/tokens';

const AVATAR = 36;
const ROW_MIN_HEIGHT = 72;
const MARK = 18;

const SECTIONS: { key: NotificationSection; title: string }[] = [
  { key: 'today', title: 'Today' },
  { key: 'week', title: 'This week' },
  { key: 'earlier', title: 'Earlier' },
];

const bodyText = {
  fontFamily: type.body.fontFamily,
  fontSize: type.body.fontSize,
  lineHeight: type.body.lineHeight,
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
      const rest = item.text ? ` commented on your ${category}: "${item.text}"` : ` commented on your ${category}.`;
      return { name, rest, spoken: `${name}${rest}` };
    }
    case 'follow':
      return { name, rest: ' started following you.', spoken: `${name} started following you.` };
    case 'follow_request':
      return { name, rest: ' wants to follow you.', spoken: `${name} wants to follow you.` };
    case 'follow_accepted':
      return { name, rest: ' accepted your request.', spoken: `${name} accepted your request.` };
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

/**
 * The page behind the bell (05-UI-SPEC section 10, the developer's picture 11), on the Spin page's
 * stack and on the Board's. The server builds the list, its sections and every time label; this
 * page prints it in three sections (Today, This week, Earlier) with a small dot on unread rows.
 * Opening it marks everything seen, so the bell's dot goes out; the dots it loaded with stay until
 * the person leaves. A follow request row has Accept and Decline. A tap opens your own entry page
 * (a reaction or comment), People (a follow), or the Spin page (your spin is open, a bonus spin).
 * No bottom bar, so the page keeps all four safe-area edges.
 */
export function NotificationsScreen({ place = 'today' }: { place?: 'today' | 'board' }) {
  const router = useRouter();
  const { user } = useSession();
  const userId = user?.id ?? null;
  const [items, setItems] = useState<NotificationItem[] | null>(null);
  const [status, setStatus] = useState<'loading' | 'ready' | 'offline'>('loading');
  const [failure, setFailure] = useState<'network' | 'generic'>('network');
  const [busy, setBusy] = useState<string | null>(null);
  const [message, setMessage] = useState('');
  const itemsRef = useRef<NotificationItem[] | null>(null);

  const load = useCallback(async () => {
    if (userId === null) return;
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
  }, [userId]);

  useFocusEffect(
    useCallback(() => {
      void load();
    }, [load])
  );

  function goBack() {
    if (router.canGoBack()) router.back();
    else router.replace(place === 'board' ? '/(app)/board' : '/(app)/today');
  }

  function open(item: NotificationItem) {
    if (item.kind === 'reaction' || item.kind === 'comment') {
      if (item.entry_id === null) return;
      router.push({
        pathname: place === 'board' ? '/(app)/board/entry' : '/(app)/today/entry',
        params: { id: item.entry_id, via: 'notifications' },
      });
      return;
    }
    if (item.kind === 'spin_open' || item.kind === 'bonus_spin') {
      router.navigate('/(app)/today');
      return;
    }
    router.push(place === 'board' ? '/(app)/board/people' : '/(app)/profile/people');
  }

  async function answer(item: NotificationItem, accept: boolean) {
    if (item.request_id === null || busy !== null) return;
    setBusy(item.id);
    setMessage('');
    try {
      if (accept) await acceptRequest(item.request_id);
      else await declineRequest(item.request_id);
      const next = (itemsRef.current ?? []).filter((shown) => shown.id !== item.id);
      itemsRef.current = next;
      setItems(next);
    } catch (err) {
      setMessage(err instanceof ApiError && err.code === 'network_unavailable' ? err.userMessage : 'Something went wrong. Please try again.');
    } finally {
      setBusy(null);
      void load();
    }
  }

  function renderRow(item: NotificationItem) {
    const words = sentence(item);
    const spoken = `${words.spoken} ${item.at_label}${item.unread ? '. New' : ''}`;
    return (
      <View key={item.id} style={{ borderBottomWidth: 1, borderBottomColor: color.divider }}>
        <Pressable
          onPress={() => open(item)}
          accessibilityRole="button"
          accessibilityLabel={spoken}
          style={{ minHeight: ROW_MIN_HEIGHT, flexDirection: 'row', alignItems: 'flex-start', paddingVertical: space.md }}
        >
          <View accessibilityElementsHidden importantForAccessibility="no-hide-descendants">
            <RowMark item={item} />
          </View>
          <View style={{ flex: 1, marginLeft: space.md }}>
            <Text numberOfLines={3} maxFontSizeMultiplier={1.3} style={bodyText}>
              {words.name !== '' ? <Text style={{ fontFamily: type.button.fontFamily }}>{words.name}</Text> : null}
              {words.rest}
            </Text>
            {item.at_label !== '' ? (
              <View style={{ marginTop: space.xs }}>
                <AppText role="label" tone="muted">
                  {item.at_label}
                </AppText>
              </View>
            ) : null}
          </View>
          <View style={{ width: space.lg, alignItems: 'flex-end', paddingTop: (type.body.lineHeight - 8) / 2 }}>
            {item.unread ? <UnreadDot /> : null}
          </View>
        </Pressable>
        {item.kind === 'follow_request' && item.request_id !== null ? (
          <View style={{ flexDirection: 'row', marginLeft: AVATAR + space.md, paddingBottom: space.md }}>
            <Chip
              label="Accept"
              tone="strong"
              onPress={busy === null ? () => void answer(item, true) : undefined}
              accessibilityLabel={`Accept ${words.name}`}
            />
            <View style={{ marginLeft: space.sm }}>
              <Chip
                label="Decline"
                tone="outline"
                onPress={busy === null ? () => void answer(item, false) : undefined}
                accessibilityLabel={`Decline ${words.name}`}
              />
            </View>
          </View>
        ) : null}
      </View>
    );
  }

  const sections = SECTIONS.map((section) => ({
    ...section,
    rows: (items ?? []).filter((item) => item.section === section.key),
  })).filter((section) => section.rows.length > 0);

  return (
    <SafeAreaView style={{ flex: 1, backgroundColor: color.dominant }}>
      <ScrollView contentContainerStyle={{ paddingHorizontal: space.lg, paddingBottom: space.xl }}>
        <View style={{ paddingTop: space.sm }}>
          <Pressable
            onPress={goBack}
            accessibilityRole="button"
            accessibilityLabel="Back"
            hitSlop={8}
            style={{ width: minTouchTarget, height: minTouchTarget, justifyContent: 'center', alignItems: 'flex-start' }}
          >
            <Ionicons name="chevron-back" size={28} color={color.ink} />
          </Pressable>
        </View>

        <View style={{ marginTop: space.md }}>
          <AppText role="title" accessibilityRole="header">
            Notifications
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
              Loading your notifications.
            </AppText>
          ) : status === 'offline' ? (
            <StatusMessage kind={failure} onRetry={() => void load()} />
          ) : sections.length === 0 ? (
            <AppText role="label" tone="muted">
              Nothing new yet.
            </AppText>
          ) : (
            sections.map((section, index) => (
              <View key={section.key} style={{ marginTop: index === 0 ? 0 : space.xl }}>
                <Text
                  accessibilityRole="header"
                  style={{
                    fontFamily: type.label.fontFamily,
                    fontSize: type.label.fontSize,
                    lineHeight: type.label.lineHeight,
                    letterSpacing: 1.5,
                    textTransform: 'uppercase',
                    color: color.muted,
                  }}
                >
                  {section.title}
                </Text>
                <View style={{ marginTop: space.sm }}>{section.rows.map(renderRow)}</View>
              </View>
            ))
          )}
        </View>
      </ScrollView>
    </SafeAreaView>
  );
}

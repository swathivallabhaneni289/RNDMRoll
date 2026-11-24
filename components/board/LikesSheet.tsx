import { useCallback, useEffect, useRef, useState } from 'react';
import { FlatList, Modal, Pressable, useWindowDimensions, View } from 'react-native';
import Animated, { Easing, useAnimatedStyle, useSharedValue, withTiming } from 'react-native-reanimated';
import { Ionicons } from '@expo/vector-icons';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { PersonRow } from '@/components/people/PeopleScreen';
import { StatusMessage } from '@/components/today/StatusMessage';
import { AppText } from '@/components/ui/AppText';
import { getPost } from '@/lib/api/board';
import { ApiError } from '@/lib/api/client';
import type { Person } from '@/lib/api/social';
import { LIKES_CLOSE, LIKES_LOADING, LIKES_TITLE, NO_LIKES, POST_GONE_LINE } from '@/lib/board/text';
import { useReduceMotion } from '@/lib/motion/useReduceMotion';
import { useOpenPerson } from '@/lib/people/openPerson';
import { color, elevation, minTouchTarget, radius, space } from '@/lib/theme/tokens';

const SLIDE_MS = 240;

/** The sheet's own read of the likes, when it is given only the post's id. */
type LikesRead =
  | { state: 'loading' }
  | { state: 'failed'; failure: 'network' | 'generic' }
  | { state: 'gone' }
  | { state: 'ready'; likes: Person[] };

const LOADING: LikesRead = { state: 'loading' };

/**
 * Who liked your own post (BRD-06, 2026-11-24), in a sheet from the bottom like the menus: each
 * person's photo, name and username, newest first as the server sends them, with nothing at the
 * right, or "No likes yet.". A tap on a person closes the sheet, then opens their page, so it
 * never opens under the dim cover. The post page and your entry page hand it the list they already
 * read (`likes`). Your posts list has only the count, so it gives `entryId`, and the sheet reads
 * the post each time it opens: the loading line, Try again when the read fails, or the gone line.
 * Under Reduce Motion it appears at once. Nothing red.
 */
export function LikesSheet({
  visible,
  onClose,
  likes,
  entryId,
}: {
  visible: boolean;
  onClose: () => void;
  likes?: Person[];
  entryId?: string;
}) {
  const reduceMotion = useReduceMotion();
  const { height } = useWindowDimensions();
  const insets = useSafeAreaInsets();
  const openPerson = useOpenPerson();
  const slideFrom = height * 0.7;
  const offset = useSharedValue(slideFrom);
  const [read, setRead] = useState<LikesRead>(LOADING);
  // Only the newest read is used: an answer for an earlier opening, or for another post, is dropped.
  const seqRef = useRef(0);
  const given = likes !== undefined;

  useEffect(() => {
    if (!visible) return;
    if (reduceMotion) {
      offset.value = 0;
      return;
    }
    offset.value = slideFrom;
    offset.value = withTiming(0, { duration: SLIDE_MS, easing: Easing.out(Easing.cubic) });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [visible]);

  const readLikes = useCallback(async (id: string) => {
    const seq = ++seqRef.current;
    setRead(LOADING);
    try {
      const post = await getPost(id);
      if (seq !== seqRef.current) return;
      setRead({ state: 'ready', likes: post.likes ?? [] });
    } catch (err) {
      if (seq !== seqRef.current) return;
      if (err instanceof ApiError && err.code === 'not_found') setRead({ state: 'gone' });
      else setRead({ state: 'failed', failure: err instanceof ApiError && err.code === 'network_unavailable' ? 'network' : 'generic' });
    }
  }, []);

  useEffect(() => {
    if (!visible) {
      // Closed: an answer still on its way is dropped, and the next opening starts at the loading line.
      ++seqRef.current;
      setRead(LOADING);
      return;
    }
    if (!given && entryId !== undefined) void readLikes(entryId);
  }, [visible, given, entryId, readLikes]);

  const anim = useAnimatedStyle(() => ({ transform: [{ translateY: offset.value }] }));

  function open(person: Person) {
    // Close the sheet first, so the page never opens under its dim cover.
    onClose();
    setTimeout(() => openPerson(person.id), 0);
  }

  const rows = likes !== undefined ? likes : read.state === 'ready' ? read.likes : null;

  return (
    <Modal visible={visible} transparent animationType="none" onRequestClose={onClose}>
      <View style={{ flex: 1, backgroundColor: color.scrimOverlay, justifyContent: 'flex-end' }}>
        <Pressable onPress={onClose} accessibilityRole="button" accessibilityLabel={LIKES_CLOSE} style={{ flex: 1 }} />
        <Animated.View
          accessibilityViewIsModal
          style={[
            {
              ...elevation.card,
              maxHeight: height * 0.7,
              borderTopLeftRadius: radius.lg,
              borderTopRightRadius: radius.lg,
              paddingBottom: insets.bottom + space.md,
            },
            anim,
          ]}
        >
          <View
            style={{
              minHeight: minTouchTarget + space.sm,
              paddingHorizontal: space.lg,
              flexDirection: 'row',
              alignItems: 'center',
              borderBottomWidth: 1,
              borderBottomColor: color.divider,
            }}
          >
            <View style={{ flex: 1 }}>
              <AppText role="heading" accessibilityRole="header">
                {LIKES_TITLE}
              </AppText>
            </View>
            <Pressable
              onPress={onClose}
              accessibilityRole="button"
              accessibilityLabel={LIKES_CLOSE}
              hitSlop={8}
              style={{ width: minTouchTarget, height: minTouchTarget, alignItems: 'flex-end', justifyContent: 'center' }}
            >
              <Ionicons name="close" size={24} color={color.ink} />
            </Pressable>
          </View>
          {rows !== null && rows.length > 0 ? (
            <FlatList
              data={rows}
              keyExtractor={(person) => person.id}
              contentContainerStyle={{ paddingHorizontal: space.lg }}
              renderItem={({ item: person, index }) => (
                <PersonRow person={person} first={index === 0} onPress={() => open(person)} pressHint="Opens their profile." />
              )}
            />
          ) : (
            <View style={{ paddingHorizontal: space.lg, paddingTop: space.lg }}>
              {rows !== null ? (
                <AppText role="label" tone="muted">
                  {NO_LIKES}
                </AppText>
              ) : read.state === 'failed' ? (
                <StatusMessage
                  kind={read.failure}
                  onRetry={() => {
                    if (entryId !== undefined) void readLikes(entryId);
                  }}
                />
              ) : read.state === 'gone' ? (
                <AppText role="body">{POST_GONE_LINE}</AppText>
              ) : (
                <AppText role="label" tone="muted">
                  {LIKES_LOADING}
                </AppText>
              )}
            </View>
          )}
        </Animated.View>
      </View>
    </Modal>
  );
}

import { useEffect, useState } from 'react';
import { FlatList, Modal, Pressable, Text, useWindowDimensions, View } from 'react-native';
import Animated, { Easing, useAnimatedStyle, useSharedValue, withTiming } from 'react-native-reanimated';
import { Ionicons } from '@expo/vector-icons';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { PersonRow } from '@/components/people/PeopleScreen';
import { AppText } from '@/components/ui/AppText';
import type { Received } from '@/lib/api/board';
import { firstName } from '@/lib/api/social';
import { REACTIONS } from '@/lib/entry/reactions';
import { useReduceMotion } from '@/lib/motion/useReduceMotion';
import { useOpenPerson } from '@/lib/people/openPerson';
import { color, elevation, minTouchTarget, radius, space, type } from '@/lib/theme/tokens';

/** At most this many different characters in the summary line, the most used first. */
const TOP = 3;
const SUMMARY_EMOJI = 18;
const ROW_EMOJI = 22;
const SLIDE_MS = 240;

type Row = { key: string; item: Received; char: string; label: string };

function rowsOf(received: Received[]): Row[] {
  return received.flatMap((item) => {
    const reaction = REACTIONS.find((r) => r.name === item.reaction);
    return reaction ? [{ key: item.person.id, item, char: reaction.char, label: reaction.label }] : [];
  });
}

/** The characters used most, ties going to the one seen first (the newest, as the server sends them). */
function topChars(rows: Row[]): string[] {
  const counts = new Map<string, number>();
  for (const row of rows) counts.set(row.char, (counts.get(row.char) ?? 0) + 1);
  return [...counts.entries()]
    .sort((a, b) => b[1] - a[1])
    .slice(0, TOP)
    .map(([char]) => char);
}

function nameOf(row: Row): string {
  return firstName(row.item.person) || row.item.person.username;
}

/** 'Sam reacted', 'Sam and Ana reacted', 'Sam and 11 others reacted'. */
function summaryText(rows: Row[]): string {
  if (rows.length === 1) return `${nameOf(rows[0])} reacted`;
  if (rows.length === 2) return `${nameOf(rows[0])} and ${nameOf(rows[1])} reacted`;
  const others = rows.length - 1;
  return `${nameOf(rows[0])} and ${others} ${others === 1 ? 'other' : 'others'} reacted`;
}

/** Everyone who reacted, one row each with their character at the right, in a sheet from the bottom. */
function ReactionsSheet({
  visible,
  rows,
  onClose,
  onOpenPerson,
}: {
  visible: boolean;
  rows: Row[];
  onClose: () => void;
  onOpenPerson: (userId: string) => void;
}) {
  const reduceMotion = useReduceMotion();
  const { height } = useWindowDimensions();
  const insets = useSafeAreaInsets();
  const slideFrom = height * 0.7;
  const offset = useSharedValue(slideFrom);

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

  const animated = useAnimatedStyle(() => ({ transform: [{ translateY: offset.value }] }));

  return (
    <Modal visible={visible} transparent animationType="none" onRequestClose={onClose}>
      <View style={{ flex: 1, backgroundColor: color.scrimOverlay, justifyContent: 'flex-end' }}>
        <Pressable onPress={onClose} accessibilityRole="button" accessibilityLabel="Close reactions" style={{ flex: 1 }} />
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
            animated,
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
                Reactions
              </AppText>
            </View>
            <Pressable
              onPress={onClose}
              accessibilityRole="button"
              accessibilityLabel="Close reactions"
              hitSlop={8}
              style={{ width: minTouchTarget, height: minTouchTarget, alignItems: 'flex-end', justifyContent: 'center' }}
            >
              <Ionicons name="close" size={24} color={color.ink} />
            </Pressable>
          </View>
          <FlatList
            data={rows}
            keyExtractor={(row) => row.key}
            contentContainerStyle={{ paddingHorizontal: space.lg }}
            renderItem={({ item: row, index }) => (
              <PersonRow
                person={row.item.person}
                first={index === 0}
                onPress={() => onOpenPerson(row.item.person.id)}
                pressHint="Opens their profile."
              >
                <Text
                  allowFontScaling={false}
                  accessibilityLabel={row.label}
                  style={{ fontSize: ROW_EMOJI, lineHeight: type.heading.lineHeight }}
                >
                  {row.char}
                </Text>
              </PersonRow>
            )}
          />
        </Animated.View>
      </View>
    </Modal>
  );
}

/**
 * Who reacted to your own post, as Instagram shows likes (developer, 2026-10-17: a list of every
 * name gets clumsy with many followers): one line with the three characters used most and 'Sam and
 * 11 others reacted'; a tap opens the sheet with everyone. With none, 'No reactions yet.'.
 */
export function ReceivedReactions({ received }: { received: Received[] }) {
  const [open, setOpen] = useState(false);
  const openPerson = useOpenPerson();
  const rows = rowsOf(received);

  if (rows.length === 0) {
    return (
      <AppText role="label" tone="muted">
        No reactions yet.
      </AppText>
    );
  }

  const text = summaryText(rows);
  return (
    <>
      <Pressable
        onPress={() => setOpen(true)}
        accessibilityRole="button"
        accessibilityLabel={`${rows.length} ${rows.length === 1 ? 'reaction' : 'reactions'}. ${text}.`}
        accessibilityHint="Opens who reacted."
        style={{ minHeight: minTouchTarget, flexDirection: 'row', alignItems: 'center' }}
      >
        <Text allowFontScaling={false} style={{ fontSize: SUMMARY_EMOJI, lineHeight: type.body.lineHeight }}>
          {topChars(rows).join('')}
        </Text>
        <View style={{ flex: 1, marginLeft: space.sm }}>
          <AppText role="body" numberOfLines={1} ellipsizeMode="tail">
            {text}
          </AppText>
        </View>
        <Ionicons name="chevron-forward" size={18} color={color.muted} />
      </Pressable>
      <ReactionsSheet
        visible={open}
        rows={rows}
        onClose={() => setOpen(false)}
        onOpenPerson={(userId) => {
          // Close the sheet first, so the page never opens under its dim cover.
          setOpen(false);
          setTimeout(() => openPerson(userId), 0);
        }}
      />
    </>
  );
}

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  BackHandler,
  Keyboard,
  KeyboardAvoidingView,
  Platform,
  Pressable,
  ScrollView,
  StyleSheet,
  TextInput,
  useWindowDimensions,
  View,
} from 'react-native';
import { Redirect, useLocalSearchParams, useRouter } from 'expo-router';
import { Ionicons } from '@expo/vector-icons';
import Animated, {
  Extrapolation,
  interpolate,
  useAnimatedStyle,
  useReducedMotion,
  useSharedValue,
  withTiming,
} from 'react-native-reanimated';
import { Gesture, GestureDetector, GestureHandlerRootView } from 'react-native-gesture-handler';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { scheduleOnRN } from 'react-native-worklets';
import { TextButton } from '@/components/ui/TextButton';
import { FadeInUp } from '@/lib/motion/primitives';
import { color, minTouchTarget, space, type } from '@/lib/theme/tokens';
import { categoryLabel, formatLong, getEntry, LabComment, LabEntry } from '@/lab/data/entries';
import { usePhoto } from '@/lab/data/photos';
import { hero } from '@/lab/motion/hero';
import { duration, easeInOut, easeOut, STAGGER } from '@/lab/motion/timing';
import { backdropWash, photoMetrics } from '@/lab/theme/frost';
import { Avatar, AVATAR_GAP, AVATAR_SIZE_SMALL } from '@/lab/components/Avatar';
import { LabPhoto } from '@/lab/components/LabPhoto';
import { LabText } from '@/lab/components/LabText';
import { StarRating } from '@/lab/components/StarRating';

const HEADER_HEIGHT = minTouchTarget;
const DISMISS_DISTANCE = 120;
const DISMISS_VELOCITY = 900;
/** The column under the photo fades out over this much pull, so only the photo follows the finger. */
const PULL_FADE_DISTANCE = 160;
/** The photo takes this share of the screen height, so the caption and thread have room below. */
const PHOTO_HEIGHT_SHARE = 0.46;
/** With the keyboard up, the photo gives way until this much is left for the thread and composer. */
const THREAD_ROOM = 232;
const MIN_PHOTO_HEIGHT = 96;
const COMMENT_MAX_LENGTH = 200;

/** A comment in the open thread. `fresh` marks one posted just now, which eases in. */
type ThreadComment = LabComment & { fresh?: boolean };

function radiusForKey(key: string): number {
  if (key.startsWith('grid:')) return photoMetrics.gridRadius;
  if (key.startsWith('diary:')) return photoMetrics.diaryRadius;
  return photoMetrics.feedRadius;
}

export default function EntryDetailScreen() {
  const { id, from } = useLocalSearchParams<{ id: string; from?: string }>();
  const entry = getEntry(id);
  if (!entry) return <Redirect href="/lab" />;
  return <EntryDetail entry={entry} sourceKey={`${from ?? 'feed'}:${entry.id}`} />;
}

function ThreadRow({ comment, first, animate }: { comment: ThreadComment; first: boolean; animate: boolean }) {
  const row = (
    <View style={{ flexDirection: 'row', gap: AVATAR_GAP, marginTop: first ? 0 : space.md }}>
      <Avatar name={comment.author} size={AVATAR_SIZE_SMALL} />
      <View style={{ flex: 1 }}>
        <LabText role="label">
          <LabText role="label" strong>
            {comment.author}
          </LabText>
          {` ${comment.text}`}
        </LabText>
        <LabText role="label" tone="muted">
          {comment.time}
        </LabText>
      </View>
    </View>
  );
  return comment.fresh ? (
    <FadeInUp duration={animate ? duration.quick : 0} distance={8}>
      {row}
    </FadeInUp>
  ) : (
    row
  );
}

/**
 * Screen C: one entry, opened, laid out like a post. The photograph grows out of the
 * list it was tapped in and stays fixed at the top with nothing on it. Under it, one
 * scrolling column holds who and when, the caption, and the full comment thread; a
 * composer is pinned to the bottom. The previous screen recedes behind a warm wash.
 * Text arrives in reading order once the photo has landed: header, title, rating,
 * caption, thread, composer.
 *
 * Presented as a transparent modal with no native animation, because this screen
 * runs the whole transition itself (see lab/motion/hero.ts). If the entry is opened
 * without a source photo (a deep link), the photo simply fades up in place. Posting a
 * comment only changes local state; nothing is saved.
 */
function EntryDetail({ entry, sourceKey }: { entry: LabEntry; sourceKey: string }) {
  const router = useRouter();
  const insets = useSafeAreaInsets();
  const { width, height } = useWindowDimensions();
  const reduced = useReducedMotion();
  const photo = usePhoto(entry);

  const origin = useMemo(() => {
    const candidate = hero.getOrigin();
    return candidate && candidate.key === sourceKey ? candidate : null;
  }, [sourceKey]);

  const photoWidth = width;
  const photoHeight = Math.min(Math.round(width / photoMetrics.aspect), Math.round(height * PHOTO_HEIGHT_SHARE));
  const photoTop = insets.top + HEADER_HEIGHT;

  const ms = useCallback((value: number) => (reduced ? 0 : value), [reduced]);

  const heroProgress = useSharedValue(origin ? 0 : 1);
  const fromRect = useSharedValue({
    x: origin?.x ?? 0,
    y: origin?.y ?? photoTop,
    width: origin?.width ?? photoWidth,
    height: origin?.height ?? photoHeight,
    radius: origin?.radius ?? 0,
  });
  const photoFade = useSharedValue(origin ? 1 : 0);
  const backdrop = useSharedValue(0);
  const content = useSharedValue(1);
  const dragY = useSharedValue(0);
  /** Height of the photo right now: full size, or shorter while the keyboard is up. */
  const photoH = useSharedValue(photoHeight);

  // Hide the source photo only after the flying photo has actually painted (two frames
  // after mount): it starts exactly on top of the source, so the swap is invisible.
  // Hiding it in the mounting commit left one blank frame on iOS, where the presented
  // modal paints a frame later than the list underneath re-renders.
  useEffect(() => {
    let inner = 0;
    const outer = requestAnimationFrame(() => {
      inner = requestAnimationFrame(() => hero.hide(sourceKey));
    });
    return () => {
      cancelAnimationFrame(outer);
      cancelAnimationFrame(inner);
    };
  }, [sourceKey]);
  useEffect(() => () => hero.release(), []);

  // Milliseconds after mount at which the photo has (nearly) arrived and the text begins.
  const landed = origin ? duration.hero : 120;

  useEffect(() => {
    if (origin) {
      heroProgress.value = withTiming(1, { duration: ms(duration.hero), easing: easeOut });
    } else {
      photoFade.value = withTiming(1, { duration: ms(duration.base), easing: easeOut });
    }
    backdrop.value = withTiming(1, { duration: ms(duration.base), easing: easeOut });
    // Mount-only: this screen plays its entrance once.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const closing = useRef(false);

  // The photo band follows the window size, except while the keyboard is up.
  const keyboardUp = useRef(false);
  const [keyboardOpen, setKeyboardOpen] = useState(false);
  useEffect(() => {
    if (!keyboardUp.current) photoH.value = photoHeight;
  }, [photoHeight, photoH]);

  // With the keyboard up there is no room for the photo at full height and the
  // composer both, so the photo shortens (a shorter crop of the same picture) and the
  // thread takes the space. It grows back when the keyboard leaves.
  useEffect(() => {
    const ios = Platform.OS === 'ios';
    const show = Keyboard.addListener(ios ? 'keyboardWillShow' : 'keyboardDidShow', (event) => {
      if (closing.current) return;
      keyboardUp.current = true;
      setKeyboardOpen(true);
      const room = event.endCoordinates.screenY - photoTop - THREAD_ROOM;
      photoH.value = withTiming(Math.max(MIN_PHOTO_HEIGHT, Math.min(photoHeight, room)), {
        duration: ms(duration.base),
        easing: easeOut,
      });
    });
    const hide = Keyboard.addListener(ios ? 'keyboardWillHide' : 'keyboardDidHide', () => {
      if (closing.current) return;
      keyboardUp.current = false;
      setKeyboardOpen(false);
      photoH.value = withTiming(photoHeight, { duration: ms(duration.base), easing: easeOut });
    });
    return () => {
      show.remove();
      hide.remove();
    };
  }, [photoTop, photoHeight, ms, photoH]);

  const finish = useCallback(() => {
    hero.release();
    router.back();
  }, [router]);

  const close = useCallback(async () => {
    if (closing.current) return;
    closing.current = true;
    Keyboard.dismiss();

    const rect = await hero.measureSource(sourceKey);
    const onScreen =
      rect !== null && rect.y + rect.height > 0 && rect.y < height && rect.x + rect.width > 0 && rect.x < width;

    content.value = withTiming(0, { duration: ms(duration.quick), easing: easeOut });
    backdrop.value = withTiming(0, { duration: ms(duration.hero), easing: easeInOut });
    dragY.value = withTiming(0, { duration: ms(duration.hero), easing: easeInOut });

    if (rect && onScreen) {
      // Travel back to wherever the source photo is now, not where it was when opened.
      fromRect.value = {
        x: rect.x,
        y: rect.y,
        width: rect.width,
        height: rect.height,
        radius: origin?.radius ?? radiusForKey(sourceKey),
      };
      heroProgress.value = withTiming(0, { duration: ms(duration.hero), easing: easeInOut }, (finished) => {
        if (finished) scheduleOnRN(finish);
      });
    } else {
      photoFade.value = withTiming(0, { duration: ms(duration.base), easing: easeOut }, (finished) => {
        if (finished) scheduleOnRN(finish);
      });
    }
  }, [sourceKey, height, width, ms, content, backdrop, dragY, fromRect, heroProgress, photoFade, origin, finish]);

  useEffect(() => {
    const subscription = BackHandler.addEventListener('hardwareBackPress', () => {
      void close();
      return true;
    });
    return () => subscription.remove();
  }, [close]);

  // The pan gesture is attached to the photo and the close row only (see the JSX), so
  // the thread below scrolls normally and a drag there never dismisses.
  const snapMs = reduced ? 0 : duration.quick;
  const pan = useMemo(
    () =>
      Gesture.Pan()
        .activeOffsetY(10)
        .failOffsetX([-30, 30])
        .onUpdate((event) => {
          dragY.value = Math.max(0, event.translationY);
        })
        .onEnd((event) => {
          if (event.translationY > DISMISS_DISTANCE || event.velocityY > DISMISS_VELOCITY) {
            scheduleOnRN(close);
          } else {
            dragY.value = withTiming(0, { duration: snapMs, easing: easeOut });
          }
        }),
    [close, dragY],
  );

  const heroStyle = useAnimatedStyle(() => {
    const p = heroProgress.value;
    const from = fromRect.value;
    return {
      left: interpolate(p, [0, 1], [from.x, 0], Extrapolation.CLAMP),
      top: interpolate(p, [0, 1], [from.y, photoTop], Extrapolation.CLAMP),
      width: interpolate(p, [0, 1], [from.width, photoWidth], Extrapolation.CLAMP),
      height: interpolate(p, [0, 1], [from.height, photoH.value], Extrapolation.CLAMP),
      borderRadius: interpolate(p, [0, 1], [from.radius, photoMetrics.detailRadius], Extrapolation.CLAMP),
      opacity: photoFade.value,
    };
  });
  // The close row and the photo, from the top edge down: the pan gesture's region, and
  // the same height of empty space at the top of the column under it.
  const bandStyle = useAnimatedStyle(() => ({ height: photoTop + photoH.value }));
  const backdropStyle = useAnimatedStyle(() => ({
    opacity: backdrop.value * (1 - Math.min(dragY.value / 420, 0.7)),
  }));
  const dragStyle = useAnimatedStyle(() => ({
    transform: [{ translateY: dragY.value }, { scale: 1 - Math.min(dragY.value / 1800, 0.04) }],
  }));
  const contentStyle = useAnimatedStyle(() => ({
    opacity: content.value * (1 - Math.min(dragY.value / PULL_FADE_DISTANCE, 1)),
  }));
  const closeStyle = useAnimatedStyle(() => ({ opacity: backdrop.value * content.value }));

  // Reading order, counted from the screen opening and starting once the photo lands.
  const at = (step: number) => ms(landed + 60 + step * STAGGER);
  const textDuration = ms(duration.base);

  // The thread and the composer. Posting only changes this screen's own state.
  const [comments, setComments] = useState<ThreadComment[]>(entry.comments);
  const [draft, setDraft] = useState('');
  const scroll = useRef<ScrollView>(null);
  const stickToEnd = useRef(false);
  const posted = useRef(0);
  const [composerFocused, setComposerFocused] = useState(false);
  const canPost = draft.trim().length > 0;

  const post = useCallback(() => {
    const text = draft.trim();
    if (text.length === 0) return;
    posted.current += 1;
    stickToEnd.current = true;
    setComments((current) => [
      ...current,
      { id: `${entry.id}-you-${posted.current}`, author: 'You', text, time: 'Just now', fresh: true },
    ]);
    setDraft('');
  }, [draft, entry.id]);

  // The new row has no height until it has laid out, so scroll when the content grows,
  // not when the state changes.
  const handleContentSizeChange = useCallback(() => {
    if (!stickToEnd.current) return;
    stickToEnd.current = false;
    scroll.current?.scrollToEnd({ animated: !reduced });
  }, [reduced]);

  return (
    // Modal content is its own native window on Android, outside the layout's gesture
    // root, so the pull-down dismiss needs a gesture root of its own here.
    <GestureHandlerRootView style={{ flex: 1 }} accessibilityViewIsModal testID="entry-detail">
      <Animated.View
        style={[StyleSheet.absoluteFill, { backgroundColor: backdropWash, pointerEvents: 'none' }, backdropStyle]}
      />

      <Animated.View style={[StyleSheet.absoluteFill, dragStyle]}>
        <Animated.View style={[{ flex: 1 }, contentStyle]}>
          <Animated.View style={bandStyle} />
          {/* Its frame must end at the bottom of the screen, which it does: it sits in a
              full-height column under the photo band. */}
          <KeyboardAvoidingView behavior={Platform.OS === 'ios' ? 'padding' : undefined} style={{ flex: 1 }}>
            <ScrollView
              ref={scroll}
              testID="entry-thread"
              keyboardShouldPersistTaps="handled"
              // On the web "on-drag" also fires for the scroll that follows a post, which would drop the focus.
              keyboardDismissMode={Platform.OS === 'ios' ? 'interactive' : Platform.OS === 'android' ? 'on-drag' : 'none'}
              onContentSizeChange={handleContentSizeChange}
              showsVerticalScrollIndicator={false}
              style={{ flex: 1 }}
              contentContainerStyle={{
                paddingHorizontal: space.lg,
                paddingTop: space.md,
                paddingBottom: space.lg,
              }}
            >
              <FadeInUp delay={at(0)} duration={textDuration} distance={8}>
                <View style={{ flexDirection: 'row', alignItems: 'center', gap: AVATAR_GAP }}>
                  <Avatar name={entry.author} />
                  <View style={{ flex: 1 }}>
                    <View
                      style={{
                        flexDirection: 'row',
                        alignItems: 'baseline',
                        justifyContent: 'space-between',
                        gap: space.sm,
                      }}
                    >
                      <LabText role="button" numberOfLines={1} style={{ flexShrink: 1 }}>
                        {entry.author}
                      </LabText>
                      <LabText role="label" tone="muted">
                        {entry.rolledAt}
                      </LabText>
                    </View>
                    <LabText role="label" tone="muted">
                      {`${categoryLabel(entry.category)} · ${formatLong(entry.date)}`}
                    </LabText>
                  </View>
                </View>
              </FadeInUp>

              <FadeInUp delay={at(1)} duration={textDuration} distance={8} style={{ marginTop: space.md }}>
                <LabText role="heading">{entry.title}</LabText>
              </FadeInUp>
              <FadeInUp delay={at(2)} duration={textDuration} distance={8} style={{ marginTop: space.xs }}>
                <StarRating value={entry.rating} size={16} />
              </FadeInUp>
              <FadeInUp delay={at(3)} duration={textDuration} distance={8} style={{ marginTop: space.sm }}>
                <LabText role="body">
                  <LabText role="body" strong>
                    {entry.author}
                  </LabText>
                  {` ${entry.reaction}`}
                </LabText>
              </FadeInUp>

              <FadeInUp delay={at(4)} duration={textDuration} distance={8} style={{ marginTop: space.md }}>
                <View style={{ height: StyleSheet.hairlineWidth, backgroundColor: color.divider }} />
                <View testID="entry-comments" style={{ marginTop: space.md }}>
                  {comments.length === 0 ? (
                    <LabText role="label" tone="muted">
                      No comments yet
                    </LabText>
                  ) : (
                    comments.map((comment, index) => (
                      <ThreadRow key={comment.id} comment={comment} first={index === 0} animate={!reduced} />
                    ))
                  )}
                </View>
              </FadeInUp>
            </ScrollView>

            <FadeInUp delay={at(5)} duration={textDuration} distance={0}>
              <View
                style={{
                  flexDirection: 'row',
                  alignItems: 'center',
                  gap: space.md,
                  paddingHorizontal: space.lg,
                  paddingTop: space.xs,
                  paddingBottom: space.xs + (keyboardOpen ? 0 : insets.bottom),
                  borderTopWidth: StyleSheet.hairlineWidth,
                  // The hairline darkens while typing: it stands in for the browser's own focus ring.
                  borderTopColor: composerFocused ? color.ink : color.divider,
                  backgroundColor: color.dominant,
                }}
              >
                <TextInput
                  value={draft}
                  onChangeText={setDraft}
                  onSubmitEditing={post}
                  onFocus={() => setComposerFocused(true)}
                  onBlur={() => setComposerFocused(false)}
                  submitBehavior="submit"
                  // react-native-web only reads the older prop; without it the field loses focus on Enter.
                  blurOnSubmit={false}
                  placeholder="Add a comment"
                  placeholderTextColor={color.muted}
                  returnKeyType="send"
                  maxLength={COMMENT_MAX_LENGTH}
                  accessibilityLabel="Add a comment"
                  testID="entry-composer-input"
                  style={{
                    flex: 1,
                    minHeight: minTouchTarget,
                    fontFamily: type.body.fontFamily,
                    fontSize: type.body.fontSize,
                    lineHeight: type.body.lineHeight,
                    color: color.ink,
                    // No ring of its own on the web: Chrome's default "auto" outline ignores the
                    // width, so the style is set explicitly to a solid one of no width.
                    outlineStyle: 'solid',
                    outlineWidth: 0,
                    paddingVertical: 0,
                    paddingHorizontal: 0,
                    textAlignVertical: 'center',
                  }}
                />
                <View testID="entry-composer-post">
                  <TextButton label="Post" disabled={!canPost} onPress={post} />
                </View>
              </View>
            </FadeInUp>
          </KeyboardAvoidingView>
        </Animated.View>

        {/* Last in the tree so the flying photo passes over the column, whatever it
            holds. Only this band takes the pull-down, never the thread. */}
        <GestureDetector gesture={pan}>
          <Animated.View style={[{ position: 'absolute', top: 0, left: 0, right: 0 }, bandStyle]}>
            <Animated.View
              accessible
              accessibilityRole="image"
              accessibilityLabel={`Photo for ${entry.title}`}
              style={[{ position: 'absolute', overflow: 'hidden', backgroundColor: color.secondary }, heroStyle]}
            >
              <LabPhoto photo={photo} />
            </Animated.View>

            <Animated.View
              style={[{ position: 'absolute', left: space.sm, top: insets.top, height: HEADER_HEIGHT }, closeStyle]}
            >
              <Pressable
                onPress={() => {
                  void close();
                }}
                accessibilityRole="button"
                accessibilityLabel="Close"
                testID="entry-close"
                style={{ width: minTouchTarget, height: HEADER_HEIGHT, alignItems: 'center', justifyContent: 'center' }}
              >
                <Ionicons name="close" size={26} color={color.ink} />
              </Pressable>
            </Animated.View>
          </Animated.View>
        </GestureDetector>
      </Animated.View>
    </GestureHandlerRootView>
  );
}

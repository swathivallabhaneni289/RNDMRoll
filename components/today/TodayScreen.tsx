import { ReactNode, useCallback, useEffect, useRef, useState } from 'react';
import { AccessibilityInfo, Pressable, Text, useWindowDimensions, View } from 'react-native';
import Animated, { Easing, useAnimatedStyle, useSharedValue, withTiming } from 'react-native-reanimated';
import { scheduleOnRN } from 'react-native-worklets';
import { useRouter } from 'expo-router';
import { AppText } from '@/components/ui/AppText';
import { ConfirmSheet } from '@/components/ui/ConfirmSheet';
import { Screen } from '@/components/ui/Screen';
import { Wheel } from '@/components/wheel/Wheel';
import { CapsLabel } from '@/components/today/CapsLabel';
import { CountdownBlock } from '@/components/today/CountdownBlock';
import { DayActions } from '@/components/today/DayActions';
import { RESULT_WORD_TOP, ResultBlock } from '@/components/today/ResultBlock';
import { SettledBlock } from '@/components/today/SettledBlock';
import { StatusMessage, type StatusKind } from '@/components/today/StatusMessage';
import { WeeklyCard } from '@/components/today/WeeklyCard';
import { BellButton } from '@/components/notifications/BellButton';
import type { Today, TodayState } from '@/lib/api/types';
import { nextSpinLine } from '@/lib/entry/rules';
import { useReduceMotion } from '@/lib/motion/useReduceMotion';
import { color, minTouchTarget, space, type } from '@/lib/theme/tokens';
import { dateLineLong } from '@/lib/wheel/format';
import { useToday } from '@/lib/wheel/useToday';
import { useUnread } from '@/lib/notifications/unreadStore';

/** The wheel is the smaller of this and the screen width minus the margins and 24dp (UI-SPEC 3). */
const WHEEL_MAX = 232;
/** The failsafe: a turn is 4 seconds, so a wheel that has not reported by now is not coming. */
const LANDING_FAILSAFE_MS = 6000;
/** After the landing nothing moves for this long, then the reveal runs (UI-SPEC 5). */
const CALM_MS = 600;
/** The fades: the wheel out, the heading swap, a bonus spin's wheel in. */
const FADE_MS = 400;

const HEADING_LINES_RESERVED = 2;
const HEADING_RESERVE = HEADING_LINES_RESERVED * type.title.lineHeight;
/** How far a one-line heading falls short of the reserved space the wheel composition keeps. */
const HEADING_DIFF = HEADING_RESERVE - type.title.lineHeight;

/**
 * idle      nothing is moving.
 * entering  a bonus spin was confirmed: the word fades out and the wheel fades in (400ms).
 * turning   the wheel has the server's answer and is turning (also the render of the landing).
 * calm      the wheel has landed; nothing moves for 600ms.
 * revealing the wheel fades out while the word rises in (400ms).
 */
type Phase = 'idle' | 'entering' | 'turning' | 'calm' | 'revealing';

function isResultState(state: TodayState | null): boolean {
  return state === 'result' || state === 'late' || state === 'posted' || state === 'skipped';
}

/**
 * The heading for a state (02-UI-SPEC 4.1 and 03-UI-SPEC 3.1). Nothing while loading or offline
 * with no data. One case for every state, so a new state cannot slip through unnamed.
 */
function headingFor(state: TodayState | null): string {
  switch (state) {
    case 'waiting':
      return 'Your next spin is coming.';
    case 'open':
      return 'Your spin is open.';
    case 'result':
      return "Today's spin.";
    case 'late':
      return "Last night's spin.";
    case 'posted':
      return 'Posted.';
    case 'skipped':
      return 'Skipped.';
    case null:
      return '';
  }
}

/** A box whose opacity runs from `from` to `to` once, when it appears. */
function FadeBox({
  from,
  to,
  children,
  absolute = false,
  top = 0,
}: {
  from: number;
  to: number;
  children: ReactNode;
  absolute?: boolean;
  top?: number;
}) {
  const opacity = useSharedValue(from);
  useEffect(() => {
    opacity.value = withTiming(to, { duration: FADE_MS, easing: Easing.out(Easing.cubic) });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
  const animated = useAnimatedStyle(() => ({ opacity: opacity.value }));
  return (
    <Animated.View
      pointerEvents={absolute ? 'none' : 'auto'}
      accessibilityElementsHidden={absolute}
      importantForAccessibility={absolute ? 'no-hide-descendants' : 'auto'}
      style={[
        absolute ? { position: 'absolute', left: 0, right: 0, top } : null,
        animated,
      ]}
    >
      {children}
    </Animated.View>
  );
}

/**
 * Posted and Skipped swap in and out at once (the developer: fades between Today states look
 * weird). Every other change of heading keeps Phase 2's fade.
 */
const SETTLED_HEADINGS: readonly string[] = ['Posted.', 'Skipped.'];

/** The heading. A new text fades out and in (the existing 400ms swap), at once under Reduce Motion. */
function Heading({ text, reduceMotion, lines }: { text: string; reduceMotion: boolean; lines: number }) {
  const [shown, setShown] = useState(text);
  const opacity = useSharedValue(1);

  useEffect(() => {
    if (text === shown) return;
    if (
      reduceMotion ||
      shown === '' ||
      text === '' ||
      SETTLED_HEADINGS.includes(text) ||
      SETTLED_HEADINGS.includes(shown)
    ) {
      setShown(text);
      return;
    }
    opacity.value = withTiming(0, { duration: FADE_MS / 2 }, (finished) => {
      'worklet';
      if (finished) scheduleOnRN(setShown, text);
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [text]);

  useEffect(() => {
    opacity.value = reduceMotion ? 1 : withTiming(1, { duration: FADE_MS / 2 });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [shown]);

  const animated = useAnimatedStyle(() => ({ opacity: opacity.value }));
  return (
    <Animated.View style={[{ minHeight: lines * type.title.lineHeight }, animated]}>
      {shown !== '' ? (
        <AppText role="title" accessibilityRole="header">
          {shown}
        </AppText>
      ) : null}
    </Animated.View>
  );
}

/**
 * The Today screen (02-UI-SPEC section 4, from the developer's picture layout 1a and the screen
 * after the spin). The phone shows what the server says: every date, window, winner and time on
 * this screen was written by the server.
 *
 * Two compositions, chosen by the server's `state`:
 *  - the wheel states (loading, waiting, open, spinning, offline): the wheel, the state block
 *    (slot A a small-caps label, slot B the countdown, slot C a Label line), the Edit wheel link;
 *  - the day states (result, late, posted, skipped): no wheel at all. Result and Late show the
 *    word with the time it was spun and `DayActions` under it (Log it, and the late line); Posted
 *    shows the entry as a row and Skipped the struck word under its sentence (`SettledBlock`).
 * The week section is last in both. A spin that was already made when the screen arrived shows
 * the day composition at once and animates nothing; a spin made here turns the wheel, waits
 * 600ms, then fades the wheel out while the word rises in (section 5). A bonus spin brings the
 * wheel back on the previous spin's own wheel and runs the same turn and reveal again. When
 * Result or Late turns into Posted or Skipped while the screen is open, the new state shows at
 * once (no cross-fade) and is announced once (03-UI-SPEC 14).
 */
export function TodayScreen() {
  const router = useRouter();
  const { width } = useWindowDimensions();
  const reduceMotion = useReduceMotion();
  const { userId, snapshot, spin, bonus, landed, tryAgain, refetch, instantReached } = useToday();
  // The dot on the bell, asked again each time this page comes into focus (Phase 5).
  const unread = useUnread();
  const { today, anchor, loadFailure, requesting, spinFailure, spinning, turn } = snapshot;

  const [phase, setPhase] = useState<Phase>('idle');
  const [bonusActive, setBonusActive] = useState(false);
  const [bonusSheetOpen, setBonusSheetOpen] = useState(false);
  const [animateResult, setAnimateResult] = useState(false);
  const bonusAfterSeqRef = useRef(0);
  const enteringBaseIdRef = useRef(0);

  // Stable callbacks, so the memoised wheel does not redraw on every state change here.
  const spinRef = useRef(spin);
  spinRef.current = spin;
  const landedRef = useRef(landed);
  landedRef.current = landed;
  const handleHubPress = useCallback(() => void spinRef.current(), []);
  const handleLanded = useCallback(() => landedRef.current(), []);

  // While the wheel turns and during the calm after it, refetch results are held: the screen
  // keeps the Today it had when the tap was made, and applies the newest on the reveal.
  // Under Reduce Motion there is no turn to wait for: the wheel is not drawn and the word shows at once.
  const holding = !reduceMotion && (spinning || phase === 'turning' || phase === 'calm');
  const stableRef = useRef<Today | null>(null);
  if (!holding) stableRef.current = today;
  const view: Today | null = holding ? (stableRef.current ?? today) : today;
  const viewState: TodayState | null = view?.state ?? null;

  const forceWheel = !reduceMotion && (bonusActive || spinning || phase === 'turning' || phase === 'calm');
  const comp: 'wheel' | 'result' = forceWheel ? 'wheel' : isResultState(viewState) ? 'result' : 'wheel';
  const compRef = useRef(comp);
  compRef.current = comp;

  // ---- Posted and Skipped (03-UI-SPEC 3.1 and 14) -----------------------------------------

  // Announce 'Posted.' or 'Skipped.' when the state CHANGES to it while the screen is open: never
  // on the first load, so a relaunch on a settled day says nothing.
  const lastStateRef = useRef<TodayState | null>(null);
  useEffect(() => {
    const before = lastStateRef.current;
    lastStateRef.current = viewState;
    if (before === null || before === viewState) return;
    if (viewState === 'posted') AccessibilityInfo.announceForAccessibility('Posted.');
    else if (viewState === 'skipped') AccessibilityInfo.announceForAccessibility('Skipped.');
  }, [viewState]);

  // ---- landing, calm and reveal ------------------------------------------------------------

  // The answer arrived: the screen is turning (and keeps its held Today until the reveal).
  useEffect(() => {
    if (spinning && phase === 'idle') setPhase('turning');
  }, [spinning, phase]);

  // The wheel landed. Under Reduce Motion (or with no wheel on screen) the result simply appears.
  useEffect(() => {
    if (phase === 'turning' && !spinning) {
      const name = today?.spin?.category.name;
      if (name) AccessibilityInfo.announceForAccessibility(`Your spin is ${name}.`);
      setPhase(!reduceMotion && compRef.current === 'wheel' ? 'calm' : 'idle');
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [phase, spinning]);

  useEffect(() => {
    if (phase === 'calm') {
      const id = setTimeout(() => {
        setBonusActive(false);
        setAnimateResult(true);
        setPhase('revealing');
      }, CALM_MS);
      return () => clearTimeout(id);
    }
    if (phase === 'revealing' || phase === 'entering') {
      const id = setTimeout(() => setPhase('idle'), FADE_MS);
      return () => clearTimeout(id);
    }
    return undefined;
  }, [phase]);

  // A turn the wheel never reports (it is not on screen, or was cancelled) must not hold the
  // screen's refetches for ever.
  useEffect(() => {
    if (!spinning) return undefined;
    const id = setTimeout(() => landed(), LANDING_FAILSAFE_MS);
    return () => clearTimeout(id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [spinning]);

  // With no wheel on screen (a bonus spin under Reduce Motion) the spin lands at once.
  useEffect(() => {
    if (spinning && comp === 'result') landed();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [spinning, comp]);

  // A failed bonus spin puts the word back, with its message under the stamp row.
  useEffect(() => {
    if (bonusActive && spinFailure && spinFailure.action === 'bonus') setBonusActive(false);
  }, [bonusActive, spinFailure]);

  // The reveal's rise belongs to the composition it ran for; a later state change shows plainly.
  useEffect(() => {
    if (comp !== 'result' && animateResult && phase === 'idle') setAnimateResult(false);
  }, [comp, animateResult, phase]);

  // ---- actions -----------------------------------------------------------------------------

  function openBonusSheet() {
    bonusAfterSeqRef.current = view?.spin?.seq ?? 0;
    setBonusSheetOpen(true);
  }

  function startBonus() {
    setBonusSheetOpen(false);
    if (!reduceMotion) {
      enteringBaseIdRef.current = turn.id;
      setBonusActive(true);
      setPhase('entering');
    }
    void bonus(bonusAfterSeqRef.current);
  }

  const busy = spinning || requesting !== null || phase !== 'idle';
  const failureKind: StatusKind | null = spinFailure ? spinFailure.kind : null;

  // ---- the wheel's props -------------------------------------------------------------------

  const turnShown = spinning || phase === 'turning' || phase === 'calm' || phase === 'revealing';
  const wheelEntries = turnShown && turn.entries ? turn.entries : (view?.wheel ?? []);
  const wheelWinner = turnShown ? turn.winnerIndex : (view?.spin ? view.spin.winner_index : null);
  const wheelSpinId = phase === 'entering' ? enteringBaseIdRef.current : turn.id;
  const wheelSize = Math.min(WHEEL_MAX, width - 2 * space.lg);
  const hubOpen = viewState === 'open' && !busy && comp === 'wheel';

  // ---- pieces ------------------------------------------------------------------------------

  const headerRow = (
    <View
      style={{
        height: minTouchTarget,
        flexDirection: 'row',
        alignItems: 'center',
        justifyContent: 'space-between',
      }}
    >
      <AppText role="heading">RNDMRoll</AppText>
      <BellButton unread={unread} onPress={() => router.push('/(app)/today/notifications')} />
    </View>
  );

  const editLinkShown = comp === 'wheel' && !bonusActive && today !== null && (viewState === 'waiting' || viewState === 'open');
  const editLink = editLinkShown ? (
    <Pressable
      onPress={() => router.push('/(app)/today/edit-wheel')}
      disabled={busy}
      accessibilityRole="button"
      accessibilityState={{ disabled: busy }}
      hitSlop={8}
      style={{ marginTop: space.sm, minHeight: minTouchTarget, justifyContent: 'center', alignSelf: 'center' }}
    >
      <Text
        style={{
          fontFamily: type.label.fontFamily,
          fontSize: type.label.fontSize,
          lineHeight: type.label.lineHeight,
          color: busy ? color.muted : color.ink,
          textDecorationLine: 'underline',
        }}
      >
        Edit wheel
      </Text>
    </Pressable>
  ) : null;

  function labelLine(text: string): ReactNode {
    return (
      <Text
        style={{
          fontFamily: type.label.fontFamily,
          fontSize: type.label.fontSize,
          lineHeight: type.label.lineHeight,
          color: color.muted,
          textAlign: 'center',
        }}
      >
        {text}
      </Text>
    );
  }

  /** Slot A, B and C under the wheel (UI-SPEC 4.1). */
  function renderStateBlock(): ReactNode {
    if (view === null) return labelLine('Loading your wheel.');
    if (bonusActive || spinning || phase === 'turning') {
      return <CapsLabel align="center">Spinning</CapsLabel>;
    }
    if (phase === 'calm' || phase === 'revealing') {
      // Landed: nothing is said while the word is about to take the wheel's place.
      return <View style={{ height: type.label.lineHeight }} />;
    }
    if (viewState === 'waiting') {
      // After "isn't open yet" or "has closed" the refetch usually says waiting, and the message
      // stays under the countdown until "Try again" or the next answer (UI-SPEC 4.4).
      return (
        <View style={{ alignSelf: 'stretch', alignItems: 'center' }}>
          <CountdownBlock anchor={anchor} nextOpenAt={view.next_open_at} onZero={instantReached} />
          {failureKind !== null ? (
            <View style={{ marginTop: space.md, alignSelf: 'stretch' }}>
              <StatusMessage
                kind={failureKind}
                busy={requesting !== null}
                onRetry={() => tryAgain(bonusAfterSeqRef.current)}
              />
            </View>
          ) : null}
        </View>
      );
    }
    if (viewState === 'open') {
      // Open, with or without a failed spin.
      return (
        <View style={{ alignSelf: 'stretch', alignItems: 'center' }}>
          <CapsLabel align="center">Tap the centre to spin</CapsLabel>
          <View style={{ marginTop: space.xs, alignSelf: 'stretch' }}>
            {failureKind !== null ? (
              <StatusMessage
                kind={failureKind}
                busy={requesting !== null}
                onRetry={() => tryAgain(bonusAfterSeqRef.current)}
              />
            ) : (
              labelLine('Open until midnight.')
            )}
          </View>
        </View>
      );
    }
    // Result, Late, Posted and Skipped draw no wheel, so they have no block under one. A wheel
    // can still be on screen for a moment in these states (the fade-out after a spin made here),
    // and it must not say 'Tap the centre to spin' then.
    return <View style={{ height: type.label.lineHeight }} />;
  }

  const wheelComposition = (frozen: boolean) => (
    <View>
      <View style={{ marginTop: space.lg, alignItems: 'center' }}>
        <Wheel
          entries={wheelEntries}
          spinId={wheelSpinId}
          winnerIndex={wheelWinner}
          size={wheelSize}
          interactive={hubOpen && !frozen}
          onHubPress={handleHubPress}
          showPointer
          onLanded={handleLanded}
          hubHint={viewState === 'waiting' ? 'Opens at 8:00 PM.' : undefined}
        />
      </View>
      <View style={{ marginTop: space.lg, alignItems: 'center' }}>{renderStateBlock()}</View>
      {frozen ? null : editLink}
    </View>
  );

  /**
   * Result and Late (picture 02): the word, its stamp row, then the day's actions (the late line
   * and 'Log it'). `live` is false for the copy that fades out over a change: no link, no
   * handlers, nothing animating.
   */
  function dayComposition(day: Today, live: boolean): ReactNode {
    const spun = day.spin;
    if (!spun) return null;
    const dayLate = day.state === 'late';
    const bonusLinkShown = live && Boolean(day.bonus.can_use) && !busy;
    const bonusFailed = live && failureKind !== null && spinFailure?.action === 'bonus';
    return (
      <View style={{ marginTop: RESULT_WORD_TOP }}>
        <ResultBlock
          name={spun.category.name}
          spunLocalTime={spun.spun_local_time}
          late={dayLate}
          canUseBonus={bonusLinkShown}
          onUseBonus={live ? openBonusSheet : () => {}}
          animate={live && animateResult}
        />
        {bonusFailed && failureKind !== null ? (
          <View style={{ marginTop: space.lg }}>
            <StatusMessage
              kind={failureKind}
              busy={requesting !== null}
              onRetry={() => tryAgain(bonusAfterSeqRef.current)}
            />
          </View>
        ) : null}
        <DayActions
          day={day}
          userId={userId}
          late={dayLate}
          followsLink={bonusLinkShown || bonusFailed}
          disabled={!live || busy}
          animate={live && animateResult}
        />
      </View>
    );
  }

  // Posted and Skipped take the place of Result or Late at once: no cross-fade between Today
  // states (the developer: fades look weird). The row and the struck word are drawn plainly.
  const postedComposition =
    view !== null ? (
      <View style={{ marginTop: space.lg }}>
        <SettledBlock today={view} />
        {/* Once the day is settled the page counts down to the next spin. */}
        <View style={{ marginTop: space.xl, alignItems: 'center' }}>
          <CountdownBlock anchor={anchor} nextOpenAt={view.next_open_at} onZero={instantReached} />
        </View>
      </View>
    ) : null;
  // The word sits halfway between the sentence under the heading and the week section: one
  // flexible space above it and one below.
  const skippedComposition =
    view !== null ? (
      <View style={{ flexGrow: 1 }}>
        <View style={{ flexGrow: 1, minHeight: space.xl }} />
        <SettledBlock today={view} />
        <View style={{ marginTop: space.xl, alignItems: 'center' }}>
          <CountdownBlock anchor={anchor} nextOpenAt={view.next_open_at} onZero={instantReached} />
        </View>
        <View style={{ flexGrow: 1, minHeight: space.xl }} />
      </View>
    ) : null;
  const skippedLine =
    comp === 'result' && viewState === 'skipped' && view !== null
      ? nextSpinLine(view.spin?.local_date ?? view.local_date, view.local_date, view.next_open_weekday)
      : null;

  // What fades out while the other composition takes over (400ms).
  let overlay: ReactNode = null;
  if (phase === 'revealing') {
    overlay = (
      <FadeBox from={1} to={0} absolute top={HEADING_DIFF}>
        {wheelComposition(true)}
      </FadeBox>
    );
  } else if (phase === 'entering' && view !== null) {
    overlay = (
      <FadeBox from={1} to={0} absolute top={-HEADING_DIFF}>
        {dayComposition(view, false)}
      </FadeBox>
    );
  }

  const offlineOnLoad = today === null && loadFailure !== null;

  // ---- the page ----------------------------------------------------------------------------

  const headingText = offlineOnLoad ? '' : headingFor(viewState);
  const headingLines = comp === 'wheel' ? HEADING_LINES_RESERVED : 1;

  return (
    <Screen edges={['top', 'left', 'right']}>
      <View style={{ flexGrow: 1 }}>
        {headerRow}

        <View style={{ marginTop: space.md, minHeight: type.label.lineHeight }}>
          {view ? <CapsLabel>{dateLineLong(view.local_date)}</CapsLabel> : null}
        </View>

        <View style={{ marginTop: space.sm }}>
          <Heading text={headingText} reduceMotion={reduceMotion} lines={headingLines} />
        </View>

        {skippedLine !== null ? (
          <View style={{ marginTop: space.sm }}>
            <AppText role="body">{skippedLine}</AppText>
          </View>
        ) : null}

        <View style={{ flexGrow: 1 }}>
          {offlineOnLoad ? (
            <View style={{ marginTop: space.xl }}>
              <StatusMessage kind={loadFailure} onRetry={() => void refetch('manual')} />
            </View>
          ) : comp === 'result' ? (
            viewState === 'posted' ? (
              postedComposition
            ) : viewState === 'skipped' ? (
              skippedComposition
            ) : view !== null ? (
              dayComposition(view, true)
            ) : null
          ) : (
            // One instance for the whole time the wheel is on screen (it must not remount when
            // the bonus spin's fade-in ends). `from` is read once, when the wheel appears.
            <FadeBox from={phase === 'entering' ? 0 : 1} to={1}>
              {wheelComposition(false)}
            </FadeBox>
          )}
          {overlay}
          {/* Skipped centres its word between two flexible spaces of its own, so none is added here. */}
          {comp === 'result' && viewState === 'skipped' && !offlineOnLoad ? null : (
            <View style={{ flexGrow: 1, minHeight: comp === 'result' ? space.xl : space.lg }} />
          )}
        </View>

        {view ? (
          <WeeklyCard week={view.week} bonusHeld={view.bonus.held} />
        ) : null}
        <View style={{ height: space.xl }} />
      </View>

      <ConfirmSheet
        visible={bonusSheetOpen}
        title="Use your bonus spin?"
        body="You will spin again and get a different word. This uses up your bonus spin."
        confirmLabel="Spin again"
        dismissLabel="Keep this word"
        onConfirm={startBonus}
        onDismiss={() => setBonusSheetOpen(false)}
      />
    </Screen>
  );
}

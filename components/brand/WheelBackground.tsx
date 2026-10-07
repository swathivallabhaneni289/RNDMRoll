import { useEffect } from 'react';
import { StyleSheet, View, useWindowDimensions } from 'react-native';
import Animated, {
  Easing,
  useAnimatedStyle,
  useReducedMotion,
  useSharedValue,
  withRepeat,
  withTiming,
} from 'react-native-reanimated';
import { Circle, Path, Svg } from 'react-native-svg';
import { HUB_RING, WEDGE_OUT, WEDGES } from '@/components/brand/BrandMark';
import { color } from '@/lib/theme/tokens';

/**
 * The entrance-page background: a faint wheel illustration that is quietly alive. Two oversized
 * ghost wheels (with dial tick marks) bleed off opposite corners and turn in opposite directions;
 * they spin up in sympathy whenever the main wheel moves (`kick` changes), then settle back to
 * their drift. A dashed arc orbits at the right edge, a few specks drift and breathe outside the
 * content column, and the wheel categories (the five defaults plus camera) sit at the edges as tiny tracked labels, one of
 * which lights up when it is `active`. Everything is ink at low opacity so it never competes with
 * the logo or buttons, and it stands still under Reduce Motion. It replaces the dot grid on
 * Welcome and the method chooser, sits behind the content, and ignores touches.
 */
const ink = (alpha: number) => `rgba(17,17,17,${alpha})`;

/** Small deterministic generator, so the specks sit in the same places on every render. */
function seeded(seed: number) {
  let t = seed >>> 0;
  return () => {
    t += 0x6d2b79f5;
    let r = Math.imul(t ^ (t >>> 15), 1 | t);
    r ^= r + Math.imul(r ^ (r >>> 7), 61 | r);
    return ((r ^ (r >>> 14)) >>> 0) / 4294967296;
  };
}

const SPECKS = (() => {
  const rand = seeded(7);
  const out: { x: number; y: number; r: number; a: number }[] = [];
  while (out.length < 36) {
    const x = rand();
    const y = rand();
    if (x > 0.2 && x < 0.8 && y > 0.22 && y < 0.88) continue; // keep the content column clear
    out.push({ x, y, r: 0.8 + rand() * 1.2, a: 0.12 + rand() * 0.26 });
  }
  return out;
})();

/** Fractions of the screen width and height. Kept off the button band (about 52 to 72 percent of the height on the chooser, bottom 13 percent on Welcome). */
const LABELS = [
  { word: 'MOVIE', x: 0.74, y: 0.11 },
  { word: 'MEAL', x: 0.78, y: 0.3 },
  { word: 'BOOK', x: 0.065, y: 0.385 },
  { word: 'SONG', x: 0.075, y: 0.755 },
  { word: 'PLACE', x: 0.09, y: 0.83 },
  { word: 'CAMERA', x: 0.7, y: 0.755 },
] as const;

/** The wheel categories shown (the five defaults plus camera), for the screen that lights one up. */
export const WHEEL_WORDS: readonly string[] = LABELS.map((l) => l.word);

function polar(cx: number, cy: number, r: number, deg: number) {
  const a = (deg * Math.PI) / 180;
  return `${(cx + r * Math.sin(a)).toFixed(2)} ${(cy - r * Math.cos(a)).toFixed(2)}`;
}

/** Sixty dial ticks just inside the rim, longer every fifth, like the thin dial in the intro video. */
function tickPath(cx: number, cy: number, R: number) {
  let d = '';
  for (let k = 0; k < 60; k++) {
    const r0 = R * 0.93;
    const r1 = r0 - R * (k % 5 === 0 ? 0.06 : 0.03);
    d += `M ${polar(cx, cy, r0, k * 6)} L ${polar(cx, cy, r1, k * 6)} `;
  }
  return d;
}

function GhostWheel({ cx, cy, R }: { cx: number; cy: number; R: number }) {
  const out = R * WEDGE_OUT * 0.9;
  return (
    <>
      {WEDGES.map(([a0, a1], i) => (
        <Path
          key={i}
          d={`M ${cx} ${cy} L ${polar(cx, cy, out, a0)} A ${out} ${out} 0 0 1 ${polar(cx, cy, out, a1)} Z`}
          fill={ink(0.07)}
        />
      ))}
      <Circle cx={cx} cy={cy} r={R} fill="none" stroke={ink(0.17)} strokeWidth={1.4} />
      <Circle cx={cx} cy={cy} r={R * 0.93} fill="none" stroke={ink(0.11)} strokeWidth={1} />
      <Path d={tickPath(cx, cy, R)} stroke={ink(0.15)} strokeWidth={1} fill="none" />
      <Circle cx={cx} cy={cy} r={R * HUB_RING} fill={ink(0.05)} stroke={ink(0.18)} strokeWidth={1.2} />
      <Circle cx={cx} cy={cy} r={R * HUB_RING * 0.45} fill="none" stroke={ink(0.18)} strokeWidth={1} />
    </>
  );
}

const KICK_EASING = Easing.bezier(0.25, 0, 0, 1);

/** One ghost wheel: a steady drift, plus an extra spin-up each time `kick` changes. */
function SlowGhost({ cx, cy, R, rot, seconds, dir, kick }: { cx: number; cy: number; R: number; rot: number; seconds: number; dir: 1 | -1; kick: number }) {
  const reduced = useReducedMotion();
  const turn = useSharedValue(rot);
  const kickTurn = useSharedValue(0);
  useEffect(() => {
    if (reduced) return;
    turn.value = withRepeat(withTiming(rot + 360 * dir, { duration: seconds * 1000, easing: Easing.linear }), -1, false);
  }, [reduced, rot, dir, seconds, turn]);
  useEffect(() => {
    if (reduced || kick === 0) return;
    kickTurn.value = withTiming(kickTurn.value + dir * 300, { duration: 2800, easing: KICK_EASING });
  }, [kick, reduced, dir, kickTurn]);
  const style = useAnimatedStyle(() => ({ transform: [{ rotate: `${turn.value + kickTurn.value}deg` }] }));
  return (
    <Animated.View style={[{ position: 'absolute', left: cx - R, top: cy - R, width: 2 * R, height: 2 * R }, style]}>
      <Svg width={2 * R} height={2 * R}>
        <GhostWheel cx={R} cy={R} R={R - 1} />
      </Svg>
    </Animated.View>
  );
}

/** The dashed arc is a big circle centred off the right edge; turning it makes the dashes orbit. */
function OrbitArc({ cx, cy, r }: { cx: number; cy: number; r: number }) {
  const reduced = useReducedMotion();
  const turn = useSharedValue(0);
  useEffect(() => {
    if (reduced) return;
    turn.value = withRepeat(withTiming(-360, { duration: 80000, easing: Easing.linear }), -1, false);
  }, [reduced, turn]);
  const style = useAnimatedStyle(() => ({ transform: [{ rotate: `${turn.value}deg` }] }));
  return (
    <Animated.View style={[{ position: 'absolute', left: cx - r, top: cy - r, width: 2 * r, height: 2 * r }, style]}>
      <Svg width={2 * r} height={2 * r}>
        <Circle cx={r} cy={r} r={r - 1} fill="none" stroke={ink(0.18)} strokeWidth={1} strokeDasharray="5 9" />
      </Svg>
    </Animated.View>
  );
}

/** The specks float up and down a little and breathe, as one layer. */
function DriftLayer({ W, H }: { W: number; H: number }) {
  const reduced = useReducedMotion();
  const k = useSharedValue(0);
  useEffect(() => {
    if (reduced) return;
    k.value = withRepeat(withTiming(1, { duration: 7000, easing: Easing.inOut(Easing.sin) }), -1, true);
  }, [reduced, k]);
  const style = useAnimatedStyle(() => ({ opacity: 0.7 + 0.3 * k.value, transform: [{ translateY: -14 * k.value }] }));
  return (
    <Animated.View style={[StyleSheet.absoluteFill, style]}>
      <Svg width={W} height={H}>
        {SPECKS.map((sp, i) => (
          <Circle key={i} cx={sp.x * W} cy={sp.y * H} r={sp.r} fill={ink(sp.a)} />
        ))}
      </Svg>
    </Animated.View>
  );
}

/** A tiny category label that darkens and lengthens its rule when it is the one the wheel picked. */
function Label({ word, x, y, active }: { word: string; x: number; y: number; active: boolean }) {
  const k = useSharedValue(0);
  useEffect(() => {
    k.value = withTiming(active ? 1 : 0, { duration: 500, easing: Easing.out(Easing.cubic) });
  }, [active, k]);
  const textStyle = useAnimatedStyle(() => ({ opacity: 0.38 + 0.62 * k.value }));
  const ruleStyle = useAnimatedStyle(() => ({ width: 28 + 28 * k.value, opacity: 0.22 + 0.6 * k.value }));
  return (
    <View style={{ position: 'absolute', left: x, top: y, flexDirection: 'row', alignItems: 'center' }}>
      <Animated.Text style={[{ fontFamily: 'WorkSans_400Regular', fontSize: 10, letterSpacing: 3, color: color.ink }, textStyle]}>{word}</Animated.Text>
      <Animated.View style={[{ height: 1, marginLeft: 8, backgroundColor: color.ink }, ruleStyle]} />
    </View>
  );
}

export function WheelBackground({ active = null, kick = 0, corner = false }: { active?: string | null; kick?: number; corner?: boolean }) {
  const { width: W, height: H } = useWindowDimensions();
  if (corner) {
    // The profile form: one faint ghost wheel in the lower left corner, nothing else.
    return (
      <View pointerEvents="none" style={StyleSheet.absoluteFill}>
        <SlowGhost cx={0.02 * W} cy={1.0 * H} R={0.34 * W} rot={20} seconds={90} dir={1} kick={0} />
      </View>
    );
  }
  return (
    <View pointerEvents="none" style={StyleSheet.absoluteFill}>
      <SlowGhost cx={0.04 * W} cy={0.185 * H} R={0.33 * W} rot={12} seconds={45} dir={1} kick={kick} />
      <SlowGhost cx={1.02 * W} cy={0.975 * H} R={0.31 * W} rot={-28} seconds={60} dir={-1} kick={kick} />
      <OrbitArc cx={1.09 * W} cy={0.6 * H} r={0.3 * W} />
      <DriftLayer W={W} H={H} />
      {LABELS.map((l) => (
        <Label key={l.word} word={l.word} x={l.x * W} y={l.y * H} active={active === l.word} />
      ))}
    </View>
  );
}

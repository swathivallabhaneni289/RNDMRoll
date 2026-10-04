import { useEffect, useRef } from 'react';
import { Pressable, View } from 'react-native';
import Animated, {
  Easing,
  useAnimatedStyle,
  useReducedMotion,
  useSharedValue,
  withDelay,
  withTiming,
} from 'react-native-reanimated';
import { Circle, Path, Polygon, Svg } from 'react-native-svg';
import {
  HUB_DOT,
  HUB_RING,
  NOTCH_STROKE,
  POINTER_POINTS,
  POINTER_STROKE,
  WEDGE_PATHS,
} from '@/components/brand/BrandMark';
import { CategoryIcon, type Category } from '@/components/brand/CategoryRow';
import { color as tokenColor } from '@/lib/theme/tokens';

/** Where each of the eight sections is centred (degrees clockwise from the top) in the logo pose; even ones are the light wedges. */
const SECTOR_CENTERS = [22.3, 70.0, 115.2, 158.4, 199.2, 240.0, 285.8, 334.0];

/** Starts from rest, speeds up fast, then slows for a long time before it stops. */
const SPIN_EASING = Easing.bezier(0.25, 0, 0, 1);
const SPIN_MS = 2600;
/** One and a bit turns of run-up, so the first spin ends in the exact logo pose. */
const FIRST_SPIN_START = -504;
const ROLL_EASING = Easing.out(Easing.cubic);
const ROLL_DELAY_MS = 250;
const ROLL_MS = 1500;

/**
 * The brand wheel, alive. With `rollIn` it rolls in from the left like a real wheel (its turning
 * is tied to how far it has travelled, one full turn over one circumference), stops in the logo
 * pose, and then its pointer drops in. Without `rollIn` it spins once instead. Tapping it spins
 * it again and it always stops back in the logo pose. `onLanded` fires each time it comes to
 * rest. Only the disc moves; the pointer and the small gap cut round its tip are a separate fixed
 * layer painted in the surface colour, so this mark belongs on a plain surface. Reduced Motion
 * shows the finished mark and does not move it.
 */
export function SpinningMark({
  size = 240,
  color = tokenColor.ink,
  wedgeColor = tokenColor.secondary,
  background = tokenColor.dominant,
  rollIn = false,
  sectors,
  onLanded,
  onSpinStart,
}: {
  size?: number;
  color?: string;
  wedgeColor?: string;
  background?: string;
  rollIn?: boolean;
  /** One category per section (eight). When given, each section carries its drawing and every stop puts one under the pointer. */
  sectors?: readonly Category[];
  onLanded?: (word: string | null) => void;
  onSpinStart?: () => void;
}) {
  const reducedMotion = useReducedMotion();
  const radiusPx = size * 0.42; // the disc radius on screen
  const rollStart = -2 * Math.PI * radiusPx;
  const rolling = rollIn && !reducedMotion;
  const pickRef = useRef(Math.floor(Math.random() * 8));
  const poseOf = (i: number) => (sectors ? -SECTOR_CENTERS[i] : 0);
  const basePose = poseOf(pickRef.current);
  const rotation = useSharedValue(basePose + (!rolling && !reducedMotion ? FIRST_SPIN_START : 0));
  const rollX = useSharedValue(rolling ? rollStart : 0);
  const pointer = useSharedValue(rolling ? 0 : 1);
  const iconsOpacity = useSharedValue(sectors && rolling ? 0 : 1); // the drawings appear once the wheel is in place
  const autoSpin = useRef<ReturnType<typeof setTimeout> | null>(null);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const landedRef = useRef(onLanded);
  landedRef.current = onLanded;
  const startRef = useRef(onSpinStart);
  startRef.current = onSpinStart;

  function landedAfter(ms: number) {
    if (timer.current) clearTimeout(timer.current);
    timer.current = setTimeout(() => landedRef.current?.(sectors ? sectors[pickRef.current] : null), ms);
  }

  useEffect(() => {
    if (reducedMotion) {
      landedAfter(300);
    } else if (rolling) {
      startRef.current?.();
      rollX.value = withDelay(ROLL_DELAY_MS, withTiming(0, { duration: ROLL_MS, easing: ROLL_EASING }));
      pointer.value = withDelay(ROLL_DELAY_MS + ROLL_MS - 150, withTiming(1, { duration: 380, easing: Easing.out(Easing.back(1.6)) }));
      iconsOpacity.value = withDelay(ROLL_DELAY_MS + ROLL_MS - 100, withTiming(1, { duration: 450 }));
      autoSpin.current = setTimeout(() => spinAgain(), ROLL_DELAY_MS + ROLL_MS + 900);
    } else {
      startRef.current?.();
      rotation.value = withDelay(350, withTiming(basePose, { duration: SPIN_MS, easing: SPIN_EASING }));
      landedAfter(350 + SPIN_MS + 100);
    }
    return () => {
      if (timer.current) clearTimeout(timer.current);
      if (autoSpin.current) clearTimeout(autoSpin.current);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  function spinAgain() {
    if (reducedMotion) {
      landedAfter(0);
      return;
    }
    startRef.current?.();
    // Pick a different section and land with its centre under the pointer, after about two more turns.
    let next = Math.floor(Math.random() * 8);
    if (next === pickRef.current) next = (next + 1 + Math.floor(Math.random() * 7)) % 8;
    pickRef.current = next;
    const pose = poseOf(next);
    const target = Math.ceil((rotation.value + 720 - pose) / 360) * 360 + pose;
    rotation.value = withTiming(target, { duration: SPIN_MS, easing: SPIN_EASING });
    landedAfter(SPIN_MS + 100);
  }

  const discStyle = useAnimatedStyle(() => ({
    transform: [
      { translateX: rollX.value },
      { rotate: `${rotation.value + (rollX.value / radiusPx) * (180 / Math.PI)}deg` },
    ],
  }));
  const iconsStyle = useAnimatedStyle(() => ({ opacity: iconsOpacity.value }));
  const pointerStyle = useAnimatedStyle(() => ({
    opacity: pointer.value,
    transform: [{ translateY: (1 - pointer.value) * -28 }],
  }));

  // Square container; the disc (radius 1) sits at 0.5 across and 0.58 down, as in BrandMark.
  const disc = size * 0.84;
  return (
    <Pressable onPress={spinAgain} accessibilityRole="button" accessibilityLabel="Spin the wheel">
      <View style={{ width: size, height: size }}>
        <Animated.View style={[{ position: 'absolute', left: size * 0.08, top: size * 0.16, width: disc, height: disc }, discStyle]}>
          <Svg viewBox="-1.05 -1.05 2.1 2.1" width={disc} height={disc}>
            <Circle cx={0} cy={0} r={1} fill={color} />
            {WEDGE_PATHS.map((d, i) => (
              <Path key={i} d={d} fill={wedgeColor} />
            ))}
            <Circle cx={0} cy={0} r={HUB_RING} fill={wedgeColor} />
            <Circle cx={0} cy={0} r={HUB_DOT} fill={color} />
          </Svg>
          {sectors ? (
            <Animated.View pointerEvents="none" style={[{ position: 'absolute', left: 0, top: 0, width: disc, height: disc }, iconsStyle]}>
              {sectors.map((word, i) => (
                <View key={i} pointerEvents="none" style={{ position: 'absolute', left: 0, top: 0, width: disc, height: disc, transform: [{ rotate: `${SECTOR_CENTERS[i]}deg` }] }}>
                  <View style={{ position: 'absolute', left: disc / 2 - disc * 0.0625, top: disc * 0.14 }}>
                    <CategoryIcon word={word} size={disc * 0.125} color={i % 2 === 0 ? color : tokenColor.dominant} />
                  </View>
                </View>
              ))}
            </Animated.View>
          ) : null}
        </Animated.View>
        <Animated.View pointerEvents="none" style={[{ position: 'absolute', left: 0, top: 0, width: size, height: size }, pointerStyle]}>
          <Svg viewBox="-1.25 -1.45 2.5 2.5" width={size} height={size}>
            <Polygon points={POINTER_POINTS} fill={background} stroke={background} strokeWidth={NOTCH_STROKE} strokeLinejoin="round" />
            <Polygon points={POINTER_POINTS} fill={color} stroke={color} strokeWidth={POINTER_STROKE} strokeLinejoin="round" />
          </Svg>
        </Animated.View>
      </View>
    </Pressable>
  );
}

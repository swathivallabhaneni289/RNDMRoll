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
import { color as tokenColor } from '@/lib/theme/tokens';

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
  onLanded,
  onSpinStart,
}: {
  size?: number;
  color?: string;
  wedgeColor?: string;
  background?: string;
  rollIn?: boolean;
  onLanded?: () => void;
  onSpinStart?: () => void;
}) {
  const reducedMotion = useReducedMotion();
  const radiusPx = size * 0.42; // the disc radius on screen
  const rollStart = -2 * Math.PI * radiusPx;
  const rolling = rollIn && !reducedMotion;
  const rotation = useSharedValue(!rolling && !reducedMotion ? FIRST_SPIN_START : 0);
  const rollX = useSharedValue(rolling ? rollStart : 0);
  const pointer = useSharedValue(rolling ? 0 : 1);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const landedRef = useRef(onLanded);
  landedRef.current = onLanded;
  const startRef = useRef(onSpinStart);
  startRef.current = onSpinStart;

  function landedAfter(ms: number) {
    if (timer.current) clearTimeout(timer.current);
    timer.current = setTimeout(() => landedRef.current?.(), ms);
  }

  useEffect(() => {
    if (reducedMotion) {
      landedAfter(300);
    } else if (rolling) {
      startRef.current?.();
      rollX.value = withDelay(ROLL_DELAY_MS, withTiming(0, { duration: ROLL_MS, easing: ROLL_EASING }));
      pointer.value = withDelay(ROLL_DELAY_MS + ROLL_MS - 150, withTiming(1, { duration: 380, easing: Easing.out(Easing.back(1.6)) }));
      landedAfter(ROLL_DELAY_MS + ROLL_MS + 350);
    } else {
      startRef.current?.();
      rotation.value = withDelay(350, withTiming(0, { duration: SPIN_MS, easing: SPIN_EASING }));
      landedAfter(350 + SPIN_MS + 100);
    }
    return () => {
      if (timer.current) clearTimeout(timer.current);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  function spinAgain() {
    if (reducedMotion) {
      landedAfter(0);
      return;
    }
    startRef.current?.();
    // Always land back in the logo pose: the next multiple of 360 after about two more turns.
    const target = Math.ceil((rotation.value + 720) / 360) * 360;
    rotation.value = withTiming(target, { duration: SPIN_MS, easing: SPIN_EASING });
    landedAfter(SPIN_MS + 100);
  }

  const discStyle = useAnimatedStyle(() => ({
    transform: [
      { translateX: rollX.value },
      { rotate: `${rotation.value + (rollX.value / radiusPx) * (180 / Math.PI)}deg` },
    ],
  }));
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

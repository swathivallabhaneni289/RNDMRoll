import { memo, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Pressable, Text, View } from 'react-native';
import Animated, { Easing, useAnimatedStyle, useSharedValue, withTiming } from 'react-native-reanimated';
import { scheduleOnRN } from 'react-native-worklets';
import { Circle, Line, Path, Polygon, Svg } from 'react-native-svg';
import { PressScale } from '@/lib/motion/primitives';
import { readReduceMotion, useReduceMotion } from '@/lib/motion/useReduceMotion';
import { color, radius, type } from '@/lib/theme/tokens';
import {
  acrossMiddle,
  acrossRoom,
  alongMiddle,
  alongRoom,
  drawnAngles,
  HUB_FRACTION,
  labelColorFor,
  labelLine,
  labelPlacement,
  poseOf,
  RIM_WIDTH,
  targetRotation,
  toneIndexes,
  wedgeCentres,
  wedgePath,
  wedgeStarts,
  type Tone,
} from '@/lib/wheel/geometry';

/** The turn: about 4 seconds, fastest in the first second, then a long slow-down (UI-SPEC 5). */
const TURN_MS = 4000;
const TURN_EASING = Easing.bezier(0.3, 0, 0.1, 1);

/** A slim pointer (02-CONTEXT D-25; it was 16 by 12), its tip on the rim. */
const POINTER_WIDTH = 12;
const POINTER_HEIGHT = 10;
/** The transparent square over the hub that takes the tap. */
const HUB_HIT = 64;
/** The dot in the hub, as a share of the wheel's diameter (it was 0.07 before D-25). */
const HUB_DOT_FRACTION = 0.05;
/**
 * The seam between wedges, in dp, drawn in the page colour as a stroke on every wedge. Each
 * wedge is painted after the one before it, so every shared edge ends up the full width. It
 * parts two dark wedges where the last wedge of an odd count meets the first. The arc part of
 * the stroke sits under the rim, and the hub covers the centre.
 */
const SEAM_WIDTH = 1.5;

export type WheelCategoryView = { name: string; weight: number };

export type WheelProps = {
  /** The categories in order, with their weights. */
  entries: ReadonlyArray<WheelCategoryView>;
  /** A counter. The wheel turns when it rises; otherwise it is placed at rest at once. */
  spinId: number;
  /** The server's winning wedge for the spin that is shown or turning, or null with no spin. */
  winnerIndex: number | null;
  /** Outer diameter in dp, rim included. */
  size: number;
  /** The hub takes taps (Today's Open state only). */
  interactive: boolean;
  onHubPress?: () => void;
  showPointer: boolean;
  /** Called once when a turn has finished (never when it was cancelled). */
  onLanded?: () => void;
  /** Spoken hint for the hub button, for example "Opens at 8:00 PM." while waiting. */
  hubHint?: string;
  /** Hidden from VoiceOver entirely (the Edit wheel preview). */
  decorative?: boolean;
};

/** The dark wedges, the rim, the hub dot and the pointer are all the one soft charcoal (D-25). */
function toneColor(tone: Tone): string {
  return tone === 'dark' ? color.ink : color.secondary;
}

/**
 * Today's wheel (UI-SPEC section 3), drawn from the server's list and turned to the server's
 * winner. It never picks a winner and never decides whether it is shown. Layers: a rotating disc
 * (wedges, rim, words), a fixed hub with its tap area, and a fixed pointer.
 *
 * Words: across the wedge when they fit, along it when they do not, never upside down in the pose
 * the wheel stops in. That pose is the shown wedge's (the winner's, or the first wedge with no
 * spin), so the words are set from `winnerIndex` in the same render in which a turn starts.
 * Labels come from React Native text so they keep capitals by style, letter spacing and
 * `allowFontScaling={false}`.
 */
function WheelImpl({
  entries,
  spinId,
  winnerIndex,
  size,
  interactive,
  onHubPress,
  showPointer,
  onLanded,
  hubHint,
  decorative = false,
}: WheelProps) {
  const reduceMotion = useReduceMotion();
  const outerRadius = size / 2;
  const count = entries.length;
  const layoutKey = entries.map((entry) => `${entry.weight}:${entry.name}`).join('|');

  const geometry = useMemo(() => {
    const angles = drawnAngles(entries.map((entry) => entry.weight));
    return {
      angles,
      starts: wedgeStarts(angles),
      centres: wedgeCentres(angles),
      tones: toneIndexes(entries.length),
    };
    // `layoutKey` carries the content of `entries`.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [layoutKey]);

  const shownIndex = winnerIndex !== null && winnerIndex >= 0 && winnerIndex < count ? winnerIndex : 0;
  const restPose = count > 0 ? poseOf(geometry.centres[shownIndex]) : 0;

  const rotation = useSharedValue(restPose);
  /** Where the disc is, or is going to rest. Spins always start from rest. */
  const rotationRef = useRef(restPose);
  const lastSpinIdRef = useRef(spinId);
  const onLandedRef = useRef(onLanded);
  onLandedRef.current = onLanded;
  const [turning, setTurning] = useState(false);

  const handleLanded = useCallback(() => {
    setTurning(false);
    onLandedRef.current?.();
  }, []);

  useEffect(() => {
    if (spinId === lastSpinIdRef.current) {
      // Not a new spin (a first draw, a refetch, an edit, a bonus spin about to start): place the
      // disc at rest on the shown spin's winner, or on the first wedge, at once.
      rotationRef.current = restPose;
      rotation.value = restPose;
      return undefined;
    }
    lastSpinIdRef.current = spinId;
    if (winnerIndex === null || count === 0) {
      rotationRef.current = restPose;
      rotation.value = restPose;
      return undefined;
    }

    let cancelled = false;
    const target = targetRotation(rotationRef.current, poseOf(geometry.centres[shownIndex]));
    rotationRef.current = target;
    // Reduce Motion is read at the start of the turn (and `useReduceMotion` follows the setting).
    void readReduceMotion().then((reduce) => {
      if (cancelled) return;
      if (reduce) {
        rotation.value = target;
        onLandedRef.current?.();
        return;
      }
      setTurning(true);
      rotation.value = withTiming(target, { duration: TURN_MS, easing: TURN_EASING }, (finished) => {
        'worklet';
        if (finished) scheduleOnRN(handleLanded);
      });
    });
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [spinId, layoutKey, shownIndex]);

  const discStyle = useAnimatedStyle(() => ({ transform: [{ rotate: `${rotation.value}deg` }] }));

  const labels = useMemo(
    () =>
      entries.map((entry, i) => {
        const angle = geometry.angles[i];
        const centre = geometry.centres[i];
        const placement = labelPlacement(centre, angle, entry.name, outerRadius, restPose);
        const text = labelLine(entry.name, placement.direction, angle, outerRadius);
        const across = placement.direction === 'across';
        const room = across ? acrossRoom(angle, outerRadius) : alongRoom(outerRadius);
        return {
          key: i,
          text,
          centre,
          middle: across ? acrossMiddle(outerRadius) : alongMiddle(outerRadius),
          boxWidth: Math.max(24, room),
          spin: (across ? 0 : -90) + (placement.flip ? 180 : 0),
          ink: labelColorFor(geometry.tones[i]) === 'dominant' ? color.dominant : color.ink,
        };
      }),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [layoutKey, shownIndex, size]
  );

  const topPad = showPointer ? POINTER_HEIGHT : 0;
  const hubDiameter = HUB_FRACTION * size;
  const dotDiameter = HUB_DOT_FRACTION * size;
  const names = entries.map((entry) => entry.name.trim()).filter((name) => name.length > 0);
  const spoken = count > 0 ? `Wheel with ${count} categories: ${names.join(', ')}` : 'Wheel';
  const hiddenFromReader = decorative || turning || count === 0;

  const hub = (
    <Svg width={HUB_HIT} height={HUB_HIT} viewBox={`${-HUB_HIT / 2} ${-HUB_HIT / 2} ${HUB_HIT} ${HUB_HIT}`}>
      <Circle cx={0} cy={0} r={hubDiameter / 2} fill={color.dominant} />
      <Circle cx={0} cy={0} r={dotDiameter / 2} fill={color.ink} />
    </Svg>
  );
  const hubStyle = {
    position: 'absolute' as const,
    left: outerRadius - HUB_HIT / 2,
    top: topPad + outerRadius - HUB_HIT / 2,
    width: HUB_HIT,
    height: HUB_HIT,
    borderRadius: radius.md,
  };

  return (
    <View style={{ width: size, height: size + topPad }}>
      <Animated.View
        accessible={!hiddenFromReader}
        accessibilityRole="image"
        accessibilityLabel={spoken}
        accessibilityElementsHidden={hiddenFromReader}
        importantForAccessibility={hiddenFromReader ? 'no-hide-descendants' : 'yes'}
        pointerEvents="none"
        style={[{ position: 'absolute', left: 0, top: topPad, width: size, height: size }, discStyle]}
      >
        <Svg width={size} height={size} viewBox={`${-outerRadius} ${-outerRadius} ${size} ${size}`}>
          {geometry.angles.map((angle, i) => (
            <Path
              key={i}
              d={wedgePath(geometry.starts[i], geometry.starts[i] + angle, outerRadius - RIM_WIDTH / 2)}
              fill={toneColor(geometry.tones[i])}
            />
          ))}
          {/* No gaps between a dark and a light wedge (smooth, like the picture). A thin page-colour
              line is drawn only where two wedges of the SAME tone meet: the last and the first of
              an odd count, which meet at 12 o'clock. */}
          {geometry.tones.length > 1 && geometry.tones[0] === geometry.tones[geometry.tones.length - 1] ? (
            <Line
              x1={0}
              y1={0}
              x2={0}
              y2={-(outerRadius - RIM_WIDTH / 2)}
              stroke={color.dominant}
              strokeWidth={SEAM_WIDTH}
            />
          ) : null}
          <Circle
            cx={0}
            cy={0}
            r={outerRadius - RIM_WIDTH / 2}
            fill="none"
            stroke={color.ink}
            strokeWidth={RIM_WIDTH}
          />
        </Svg>
        {labels.map((label) => (
          <View
            key={label.key}
            pointerEvents="none"
            style={{
              position: 'absolute',
              left: 0,
              top: 0,
              width: size,
              height: size,
              transform: [{ rotate: `${label.centre}deg` }],
            }}
          >
            <View
              style={{
                position: 'absolute',
                left: outerRadius - label.boxWidth / 2,
                top: outerRadius - label.middle - type.wheelLabel.lineHeight / 2,
                width: label.boxWidth,
                height: type.wheelLabel.lineHeight,
                transform: [{ rotate: `${label.spin}deg` }],
              }}
            >
              <Text
                numberOfLines={1}
                allowFontScaling={false}
                style={{
                  fontFamily: type.wheelLabel.fontFamily,
                  fontSize: type.wheelLabel.fontSize,
                  lineHeight: type.wheelLabel.lineHeight,
                  letterSpacing: 1.5,
                  textTransform: 'uppercase',
                  textAlign: 'center',
                  color: label.ink,
                }}
              >
                {label.text}
              </Text>
            </View>
          </View>
        ))}
      </Animated.View>

      {showPointer ? (
        <View
          pointerEvents="none"
          accessibilityElementsHidden
          importantForAccessibility="no-hide-descendants"
          style={{ position: 'absolute', left: outerRadius - POINTER_WIDTH / 2, top: 0 }}
        >
          <Svg width={POINTER_WIDTH} height={POINTER_HEIGHT} viewBox={`0 0 ${POINTER_WIDTH} ${POINTER_HEIGHT}`}>
            <Polygon
              points={`0,0 ${POINTER_WIDTH},0 ${POINTER_WIDTH / 2},${POINTER_HEIGHT}`}
              fill={color.ink}
            />
          </Svg>
        </View>
      ) : null}

      {decorative || count === 0 ? (
        <View pointerEvents="none" style={hubStyle}>
          {hub}
        </View>
      ) : reduceMotion ? (
        <Pressable
          onPress={interactive ? onHubPress : undefined}
          disabled={!interactive}
          accessibilityRole="button"
          accessibilityLabel="Spin the wheel"
          accessibilityHint={hubHint}
          accessibilityState={{ disabled: !interactive }}
          style={hubStyle}
        >
          {hub}
        </Pressable>
      ) : (
        <PressScale
          onPress={interactive ? onHubPress : undefined}
          disabled={!interactive}
          accessibilityRole="button"
          accessibilityLabel="Spin the wheel"
          accessibilityHint={hubHint}
          accessibilityState={{ disabled: !interactive }}
          style={hubStyle}
        >
          {hub}
        </PressScale>
      )}
    </View>
  );
}

export const Wheel = memo(WheelImpl);

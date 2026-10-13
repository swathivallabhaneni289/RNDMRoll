import { useRef } from 'react';
import { Circle, Defs, G, Mask, Path, Polygon, Rect, Svg } from 'react-native-svg';
import { color as tokenColor } from '@/lib/theme/tokens';

/**
 * The wheel brand mark: a solid charcoal disc with four light wedges, a ring-and-dot hub, and a
 * fixed rounded pointer at 12 o'clock with a small gap cut round its tip. It replaced the
 * five-pip dice mark when the dice mechanic gave way to the spin wheel. The shapes are
 * measured from the developer-supplied logo and expressed in disc-radius units (disc radius
 * 1, centre 0,0). Carries no elevation, no fill container, and no border, per UI-SPEC's
 * Focal Points section: the mark must not outweigh the CTA group on the choose-method screen.
 *
 * Finer look (2026-10-10, 02-CONTEXT D-25): the wedge angles and the pointer's shape are the
 * logo's own and did not change. Only the weights did. The outer ring, the hub and the pointer
 * are drawn at the proportions of Today's wheel (a 1.5dp rim, a hub of 14 percent of the
 * diameter with a dot of 5 percent, a pointer 12dp wide on a 232dp wheel). Each weight also has
 * a least size on screen, so the mark still reads at 30dp in the Spin button (`markShape`).
 *
 * The gap round the pointer is a real cut (an SVG mask), not a background-coloured stroke,
 * so the mark sits correctly on the dot-grid texture and on any other surface.
 */
export const WEDGES: ReadonlyArray<readonly [number, number]> = [
  [-2.0, 46.6],
  [93.3, 137.0],
  [179.8, 218.5],
  [261.5, 310.1],
];

// The weights at the big size (the Welcome wheel), in disc-radius units. Each old value is in its
// comment.
/** The outer ring: the dark band between the ends of the wedges and the edge of the disc (was 0.0936). */
const RING = 0.0145;
/** The light circle of the hub (was 0.1735). The ghost wheels in `WheelBackground` read it too. */
export const HUB_RING = 0.14;
/** The dot in the hub (was 0.1102). */
const HUB_DOT = 0.05;
/** The pointer's width, its round stroke included (was 0.3764). */
const POINTER_WIDTH = 0.115;
/**
 * The pointer's own shape, measured from the logo and kept: its height over its width, the share
 * of its width that is the round stroke, the gap cut round its tip, and how far the tip bites
 * into the disc. The old pointer comes out of these four numbers at its old width.
 */
const POINTER_ASPECT = 1.009;
const POINTER_STROKE_SHARE = 0.2;
const POINTER_GAP_SHARE = 0.165;
const POINTER_BITE_SHARE = 0.11;

/** The least each weight is on screen, in dp, so a small mark (30dp in the Spin button) still reads. */
const MIN_RING_DP = 0.8;
const MIN_HUB_DP = 1.8;
const MIN_DOT_DP = 0.95;
const MIN_POINTER_DP = 4;

/** Angles are degrees clockwise from 12 o'clock. */
function polar(deg: number, r: number): string {
  const a = (deg * Math.PI) / 180;
  return `${(r * Math.sin(a)).toFixed(4)} ${(-r * Math.cos(a)).toFixed(4)}`;
}

export type MarkShape = {
  /** One path per light wedge, from the centre out to the inner edge of the ring. */
  wedgePaths: string[];
  /** Radius of the light hub circle and of its dot. */
  hubRing: number;
  hubDot: number;
  /** The pointer's triangle, to be drawn with `pointerStroke` and a round join. */
  pointerPoints: string;
  pointerStroke: number;
  /** The stroke that cuts the gap round the pointer (the pointer's stroke plus the gap on both sides). */
  notchStroke: number;
};

/**
 * Everything that has a weight, for a mark whose disc radius takes `unitDp` dp on screen: the
 * proportions above, but never finer than the least sizes. At the Welcome size none of the
 * least sizes applies, so that wheel has exactly the proportions of Today's. The angles of the
 * wedges (`WEDGES`) are not touched.
 */
export function markShape(unitDp: number): MarkShape {
  const atLeast = (units: number, minDp: number) => Math.max(units, minDp / unitDp);
  const ring = atLeast(RING, MIN_RING_DP);
  const width = atLeast(POINTER_WIDTH, MIN_POINTER_DP);
  const stroke = POINTER_STROKE_SHARE * width;
  const gap = POINTER_GAP_SHARE * width;
  // The round join pushes the drawn tip half a stroke past the corner of the triangle.
  const apex = -(1 - POINTER_BITE_SHARE * width) - stroke / 2;
  const top = apex - (POINTER_ASPECT * width - stroke);
  const half = (width - stroke) / 2;
  const out = 1 - ring;
  const [left, right, y, tipY] = [-half, half, top, apex].map((n) => n.toFixed(4));
  return {
    wedgePaths: WEDGES.map(
      ([a0, a1]) => `M 0 0 L ${polar(a0, out)} A ${out.toFixed(4)} ${out.toFixed(4)} 0 0 1 ${polar(a1, out)} Z`
    ),
    hubRing: atLeast(HUB_RING, MIN_HUB_DP),
    hubDot: atLeast(HUB_DOT, MIN_DOT_DP),
    pointerPoints: `${left},${y} ${right},${y} 0,${tipY}`,
    pointerStroke: stroke,
    notchStroke: stroke + 2 * gap,
  };
}

let markCount = 0;

export function BrandMark({
  size = 64,
  color = tokenColor.ink,
  wedgeColor = tokenColor.secondary,
}: {
  size?: number;
  color?: string;
  wedgeColor?: string;
}) {
  const idRef = useRef<string | null>(null);
  if (idRef.current === null) idRef.current = `brandmark-notch-${markCount++}`;
  const maskId = idRef.current;
  // The view box is 2.5 units wide, so the disc radius (one unit) is size / 2.5 dp.
  const shape = markShape(size / 2.5);

  return (
    <Svg viewBox="-1.25 -1.45 2.5 2.5" width={size} height={size}>
      <Defs>
        <Mask id={maskId} x="-1.25" y="-1.45" width="2.5" height="2.5" maskUnits="userSpaceOnUse">
          <Rect x="-1.25" y="-1.45" width="2.5" height="2.5" fill="white" />
          <Polygon
            points={shape.pointerPoints}
            fill="black"
            stroke="black"
            strokeWidth={shape.notchStroke}
            strokeLinejoin="round"
          />
        </Mask>
      </Defs>
      <G mask={`url(#${maskId})`}>
        <Circle cx={0} cy={0} r={1} fill={color} />
        {shape.wedgePaths.map((d, i) => (
          <Path key={i} d={d} fill={wedgeColor} />
        ))}
        <Circle cx={0} cy={0} r={shape.hubRing} fill={wedgeColor} />
        <Circle cx={0} cy={0} r={shape.hubDot} fill={color} />
      </G>
      <Polygon
        points={shape.pointerPoints}
        fill={color}
        stroke={color}
        strokeWidth={shape.pointerStroke}
        strokeLinejoin="round"
      />
    </Svg>
  );
}

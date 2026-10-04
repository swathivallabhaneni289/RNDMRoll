import { useRef } from 'react';
import { Circle, Defs, G, Mask, Path, Polygon, Rect, Svg } from 'react-native-svg';
import { color as tokenColor } from '@/lib/theme/tokens';

/**
 * The wheel brand mark: a solid Ink disc with four light wedges, a ring-and-dot hub, and a
 * fixed rounded pointer at 12 o'clock with a small gap cut round its tip. It replaced the
 * five-pip dice mark when the dice mechanic gave way to the spin wheel. The shapes are
 * measured from the developer-supplied logo and expressed in disc-radius units (disc radius
 * 1, centre 0,0). Carries no elevation, no fill container, and no border, per UI-SPEC's
 * Focal Points section: the mark must not outweigh the CTA group on the choose-method screen.
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
export const WEDGE_OUT = 0.9064;
export const HUB_RING = 0.1735;
export const HUB_DOT = 0.1102;

/** Angles are degrees clockwise from 12 o'clock. */
function polar(deg: number, r: number): string {
  const a = (deg * Math.PI) / 180;
  return `${(r * Math.sin(a)).toFixed(4)} ${(-r * Math.cos(a)).toFixed(4)}`;
}

export const WEDGE_PATHS = WEDGES.map(
  ([a0, a1]) => `M 0 0 L ${polar(a0, WEDGE_OUT)} A ${WEDGE_OUT} ${WEDGE_OUT} 0 0 1 ${polar(a1, WEDGE_OUT)} Z`
);

/** The pointer is an inset triangle drawn with a round-joined stroke, which rounds its corners. */
export const POINTER_POINTS = '-0.1507,-1.3022 0.1507,-1.3022 0,-0.9965';
export const POINTER_STROKE = 0.075;
export const NOTCH_STROKE = 0.1993;

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

  return (
    <Svg viewBox="-1.25 -1.45 2.5 2.5" width={size} height={size}>
      <Defs>
        <Mask id={maskId} x="-1.25" y="-1.45" width="2.5" height="2.5" maskUnits="userSpaceOnUse">
          <Rect x="-1.25" y="-1.45" width="2.5" height="2.5" fill="white" />
          <Polygon
            points={POINTER_POINTS}
            fill="black"
            stroke="black"
            strokeWidth={NOTCH_STROKE}
            strokeLinejoin="round"
          />
        </Mask>
      </Defs>
      <G mask={`url(#${maskId})`}>
        <Circle cx={0} cy={0} r={1} fill={color} />
        {WEDGE_PATHS.map((d, i) => (
          <Path key={i} d={d} fill={wedgeColor} />
        ))}
        <Circle cx={0} cy={0} r={HUB_RING} fill={wedgeColor} />
        <Circle cx={0} cy={0} r={HUB_DOT} fill={color} />
      </G>
      <Polygon
        points={POINTER_POINTS}
        fill={color}
        stroke={color}
        strokeWidth={POINTER_STROKE}
        strokeLinejoin="round"
      />
    </Svg>
  );
}

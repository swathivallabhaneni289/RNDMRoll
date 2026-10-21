import { useRef } from 'react';
import { Circle, Defs, Line, Mask, Polygon, Rect, Svg } from 'react-native-svg';
import { color as tokenColor } from '@/lib/theme/tokens';

/**
 * The Shutter logo (developer, 2026-10-21): six camera blades on a wheel that stops under a
 * rounded pointer, in one colour. The app icon is this mark light on near-black
 * (assets/icon.svg); in the app it sits only beside the RNDMRoll title on the Spin page. The
 * wheel's mark (`BrandMark`) is a different drawing and stays everywhere else.
 *
 * The shapes are the logo's own, in its 1000-unit drawing (wheel centre 500,540, radius 380). The
 * blades are cut out of the disc by an SVG mask, as is the gap round the pointer's tip, so the
 * mark sits correctly on any surface.
 */

/**
 * The mark's box in drawing units, centred on the wheel: from the pointer's top to as far below the
 * wheel's centre, with 4 units to spare at the sides for the edges' smoothing. Lining the mark up by
 * its centre therefore lines up the wheel, not the wheel and pointer together.
 */
const BOX = { x: 116, y: 28, width: 768, height: 1024 };
const VIEW_BOX = `${BOX.x} ${BOX.y} ${BOX.width} ${BOX.height}`;
/** The wheel's diameter in drawing units. */
const WHEEL = 760;

/** The six cuts between the blades, as [x1, y1, x2, y2]. */
const CUTS: ReadonlyArray<readonly [number, number, number, number]> = [
  [621.24, 465, 900.79, 626.4],
  [625.57, 607.5, 625.57, 930.3],
  [504.33, 682.5, 224.78, 843.9],
  [378.76, 615, 99.21, 453.6],
  [374.43, 472.5, 374.43, 149.7],
  [495.67, 397.5, 775.22, 236.1],
];
const APERTURE = '500,379.99 638.57,459.99 638.57,620.01 500,700.01 361.43,620.01 361.43,459.99';
const POINTER = '442,46 558,46 500,162';

let markCount = 0;

/** size is the wheel's diameter; the pointer rises above it and the same room is left below. */
export function ShutterMark({ size = 20, color = tokenColor.ink }: { size?: number; color?: string }) {
  const idRef = useRef<string | null>(null);
  if (idRef.current === null) idRef.current = `shutter-mask-${markCount++}`;
  const maskId = idRef.current;

  return (
    <Svg viewBox={VIEW_BOX} width={(size * BOX.width) / WHEEL} height={(size * BOX.height) / WHEEL}>
      <Defs>
        <Mask id={maskId} {...BOX} maskUnits="userSpaceOnUse">
          <Rect {...BOX} fill="black" />
          <Circle cx={500} cy={540} r={380} fill="white" />
          <Polygon points={APERTURE} fill="black" />
          {CUTS.map(([x1, y1, x2, y2], i) => (
            <Line key={i} x1={x1} y1={y1} x2={x2} y2={y2} stroke="black" strokeWidth={26} />
          ))}
          <Polygon points={POINTER} fill="black" stroke="black" strokeWidth={74} strokeLinejoin="round" />
          <Polygon points={POINTER} fill="white" stroke="white" strokeWidth={28} strokeLinejoin="round" />
        </Mask>
      </Defs>
      <Rect {...BOX} fill={color} mask={`url(#${maskId})`} />
    </Svg>
  );
}

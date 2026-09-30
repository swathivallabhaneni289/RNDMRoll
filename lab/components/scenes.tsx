import { ReactElement } from 'react';
import { Circle, Ellipse, G, Path, Polygon, Rect } from 'react-native-svg';
import { color } from '@/lib/theme/tokens';
import type { SceneId } from '@/lab/data/entries';

/**
 * Designed placeholders for photographs: flat, monochrome tonal studies drawn only
 * from the token neutrals. They exist so frosted material has something worth
 * blurring behind it and so the panel can be judged across the tonal range, from
 * near black (night) to near white (snow). They are deliberately not photographic:
 * the product allows real user photos only, so nothing here imitates stock imagery.
 *
 * Every scene is drawn on a 400 x 500 canvas (the 4:5 portrait ratio) and is meant
 * to be cropped with preserveAspectRatio "slice", exactly like a photo with
 * contentFit "cover".
 */
export const SCENE_WIDTH = 400;
export const SCENE_HEIGHT = 500;

function Backdrop({ fill }: { fill: string }) {
  return <Rect width={SCENE_WIDTH} height={SCENE_HEIGHT} fill={fill} />;
}

/** Mid-dark: a lit window, a shaft of light on the floor, a chair. */
function Window() {
  return (
    <G>
      <Backdrop fill={color.ink} />
      <Rect y={330} width={SCENE_WIDTH} height={170} fill={color.muted} fillOpacity={0.35} />
      <Polygon points="128,332 292,332 372,500 36,500" fill={color.dominant} fillOpacity={0.24} />
      <Rect x={120} y={70} width={172} height={262} fill={color.dominant} />
      <Rect x={203} y={70} width={5} height={262} fill={color.ink} />
      <Rect x={120} y={196} width={172} height={5} fill={color.ink} />
      <Rect x={108} y={332} width={196} height={10} fill={color.divider} />
      <Rect x={292} y={410} width={60} height={8} fill={color.ink} />
      <Rect x={292} y={368} width={8} height={50} fill={color.ink} />
      <Rect x={296} y={418} width={6} height={42} fill={color.ink} />
      <Rect x={344} y={418} width={6} height={42} fill={color.ink} />
    </G>
  );
}

/** Mid-light: sky, sea, sand, a small boat. */
function Horizon() {
  const glints = [0, 1, 2, 3, 4].map((i) => (
    <Rect
      key={i}
      x={286 - i * 8}
      y={262 + i * 15}
      width={20 + i * 14}
      height={3}
      fill={color.card}
      fillOpacity={0.55}
    />
  ));
  return (
    <G>
      <Backdrop fill={color.secondary} />
      <Rect y={250} width={SCENE_WIDTH} height={150} fill={color.muted} fillOpacity={0.78} />
      <Rect y={400} width={SCENE_WIDTH} height={100} fill={color.divider} />
      <Rect y={247} width={SCENE_WIDTH} height={4} fill={color.ink} fillOpacity={0.55} />
      <Circle cx={292} cy={176} r={24} fill={color.card} />
      {glints}
      <Polygon points="118,246 152,246 145,258 126,258" fill={color.ink} />
      <Rect x={134} y={226} width={2} height={20} fill={color.ink} />
      <Polygon points="138,228 138,244 152,244" fill={color.card} />
    </G>
  );
}

/** Mid, warm: a bowl on a table against a wall with a soft window streak. */
function Table() {
  return (
    <G>
      <Backdrop fill={color.divider} />
      <Polygon points="300,0 400,0 400,250 270,250" fill={color.card} fillOpacity={0.35} />
      <Rect y={250} width={SCENE_WIDTH} height={250} fill={color.muted} fillOpacity={0.55} />
      <Rect y={248} width={SCENE_WIDTH} height={4} fill={color.ink} fillOpacity={0.5} />
      <Ellipse cx={210} cy={286} rx={104} ry={14} fill={color.ink} fillOpacity={0.25} />
      <Circle cx={172} cy={240} r={26} fill={color.muted} />
      <Circle cx={220} cy={232} r={30} fill={color.card} />
      <Circle cx={254} cy={246} r={20} fill={color.secondary} />
      <Path d="M126 254 A74 74 0 0 0 274 254 Z" fill={color.ink} />
    </G>
  );
}

/** Mid-dark, symmetric: a doorway and a figure standing in it. */
function Doorway() {
  return (
    <G>
      <Backdrop fill={color.secondary} />
      <Rect x={96} y={54} width={208} height={430} fill={color.ink} />
      <Rect x={118} y={76} width={164} height={408} fill={color.divider} />
      <Rect x={118} y={76} width={164} height={44} fill={color.muted} fillOpacity={0.4} />
      <Circle cx={200} cy={252} r={18} fill={color.ink} />
      <Polygon points="180,274 220,274 228,412 172,412" fill={color.ink} />
      <Rect y={484} width={SCENE_WIDTH} height={16} fill={color.ink} fillOpacity={0.35} />
    </G>
  );
}

/** Very dark: three towers, a few lit windows, one street lamp. */
function Night() {
  const towers = [
    { x: 0, y: 150, w: 118, h: 350 },
    { x: 128, y: 96, w: 146, h: 404 },
    { x: 284, y: 176, w: 116, h: 324 },
  ];
  const windows: ReactElement[] = [];
  towers.forEach((tower, t) => {
    const columns = 3;
    const cell = (tower.w - 24) / columns;
    for (let row = 0; row < 6; row += 1) {
      for (let col = 0; col < columns; col += 1) {
        const lit = (row * 7 + col * 3 + t * 5) % 5 === 0;
        windows.push(
          <Rect
            key={`${t}-${row}-${col}`}
            x={tower.x + 12 + col * cell + 4}
            y={tower.y + 24 + row * 36}
            width={cell - 8}
            height={20}
            fill={color.dominant}
            fillOpacity={lit ? 0.92 : 0.1}
          />,
        );
      }
    }
  });
  return (
    <G>
      <Backdrop fill={color.ink} />
      {towers.map((tower) => (
        <Rect key={tower.x} x={tower.x} y={tower.y} width={tower.w} height={tower.h} fill={color.muted} fillOpacity={0.24} />
      ))}
      {windows}
      <Rect x={352} y={300} width={4} height={200} fill={color.muted} fillOpacity={0.7} />
      <Circle cx={354} cy={296} r={7} fill={color.card} />
      <Rect x={326} y={480} width={56} height={3} fill={color.card} fillOpacity={0.3} />
    </G>
  );
}

/** Very light: snow, a ridge, bare trunks, a line of footprints. */
function Snow() {
  const prints = [0, 1, 2, 3, 4, 5].map((i) => (
    <Ellipse key={i} cx={120 + i * 40} cy={430 - i * 10} rx={9} ry={4} fill={color.divider} />
  ));
  return (
    <G>
      <Backdrop fill={color.card} />
      <Rect y={350} width={SCENE_WIDTH} height={150} fill={color.secondary} />
      <Polygon points="0,352 120,330 260,354 400,336 400,362 0,362" fill={color.dominant} />
      <Rect x={90} y={196} width={8} height={166} fill={color.ink} fillOpacity={0.6} />
      <Polygon points="94,240 132,202 135,207 98,247" fill={color.ink} fillOpacity={0.5} />
      <Polygon points="94,270 60,240 57,245 92,278" fill={color.ink} fillOpacity={0.5} />
      <Rect x={230} y={230} width={6} height={130} fill={color.ink} fillOpacity={0.45} />
      <Polygon points="233,262 266,232 268,236 236,268" fill={color.ink} fillOpacity={0.4} />
      <Rect x={330} y={270} width={5} height={90} fill={color.ink} fillOpacity={0.35} />
      {prints}
    </G>
  );
}

/** Mid, busy: diagonal bands of light and shadow, like a stairwell. */
function Stairs() {
  const bands = [0, 1, 2, 3, 4, 5, 6, 7, 8, 9].map((i) => {
    const y = i * 56;
    return (
      <Polygon
        key={i}
        points={`0,${y} 400,${y - 70} 400,${y - 42} 0,${y + 28}`}
        fill={i % 2 === 0 ? color.ink : color.dominant}
        fillOpacity={0.9}
      />
    );
  });
  return (
    <G>
      <Backdrop fill={color.secondary} />
      {bands}
    </G>
  );
}

/** Mid, busy: a quilt of tone squares crossed by one diagonal shadow. */
function Tiles() {
  const tones = [color.ink, color.muted, color.divider, color.secondary, color.dominant, color.card];
  const cell = 80;
  const squares: ReactElement[] = [];
  for (let row = 0; row < 7; row += 1) {
    for (let col = 0; col < 5; col += 1) {
      const tone = tones[(row * 5 + col * 3 + ((row * col) % 4)) % tones.length];
      squares.push(<Rect key={`${row}-${col}`} x={col * cell + 1} y={row * cell + 1} width={cell - 2} height={cell - 2} fill={tone} />);
    }
  }
  return (
    <G>
      <Backdrop fill={color.divider} />
      {squares}
      <Polygon points="0,0 270,0 0,340" fill={color.ink} fillOpacity={0.16} />
    </G>
  );
}

const SCENES: Record<SceneId, () => ReactElement> = {
  window: Window,
  horizon: Horizon,
  table: Table,
  doorway: Doorway,
  night: Night,
  snow: Snow,
  stairs: Stairs,
  tiles: Tiles,
};

export function renderScene(id: SceneId): ReactElement {
  const Scene = SCENES[id];
  return <Scene />;
}

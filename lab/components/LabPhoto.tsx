import { memo, useId } from 'react';
import { StyleProp, StyleSheet, View, ViewStyle } from 'react-native';
import { Image as ExpoImage } from 'expo-image';
import Svg, { Defs, FeGaussianBlur, Filter, G, Image as SvgImage } from 'react-native-svg';
import type { PhotoRef } from '@/lab/data/photos';
import { renderScene, SCENE_HEIGHT, SCENE_WIDTH } from './scenes';

type Props = {
  photo: PhotoRef;
  style?: StyleProp<ViewStyle>;
  /**
   * Draws a blurred copy instead of the sharp photo. Gaussian sigma in dp. Needs
   * `width` and `height` (the size the photo is drawn at) so the blur radius stays
   * constant in dp whatever the source resolution.
   */
  blur?: number;
  width?: number;
  height?: number;
};

/**
 * The blurred copy is rasterised at a quarter of its size and scaled back up by the
 * compositor. A Gaussian blur discards fine detail anyway, so nothing visible is
 * lost, but the filter then runs over 1/16th of the pixels. Rendering it at full
 * size froze the photo transition for over a second on iOS, because the filter runs
 * on the main thread that also drives the animation.
 */
const BLUR_DOWNSCALE = 4;

/**
 * One photo, two sources, one crop rule. Sharp photos fill their parent with
 * "cover"; the blurred variant is what the frosted panel is made of. Both crop from
 * the centre, so the blurred copy lines up exactly with the sharp photo under it.
 */
export const LabPhoto = memo(function LabPhoto({ photo, style, blur, width, height }: Props) {
  const rawId = useId();
  const filterId = `frost${rawId.replace(/[^a-zA-Z0-9]/g, '')}`;
  const blurred = blur !== undefined && blur > 0 && width !== undefined && height !== undefined;

  if (!blurred) {
    if (photo.kind === 'uri') {
      return (
        <View style={[StyleSheet.absoluteFill, style]}>
          <ExpoImage source={{ uri: photo.uri }} contentFit="cover" style={StyleSheet.absoluteFill} />
        </View>
      );
    }
    return (
      <View style={[StyleSheet.absoluteFill, style]}>
        <Svg
          width="100%"
          height="100%"
          viewBox={`0 0 ${SCENE_WIDTH} ${SCENE_HEIGHT}`}
          preserveAspectRatio="xMidYMid slice"
        >
          {renderScene(photo.id)}
        </Svg>
      </View>
    );
  }

  const k = BLUR_DOWNSCALE;
  const smallWidth = width / k;
  const smallHeight = height / k;

  // The small SVG is scaled back up from its top-left corner to fill the full box.
  const frame = [{ width, height, overflow: 'hidden' as const }, style];
  const scaledUp = { width: smallWidth, height: smallHeight, transform: [{ scale: k }], transformOrigin: 'top left' };

  if (photo.kind === 'uri') {
    return (
      <View style={frame}>
        <View style={scaledUp}>
          <Svg width={smallWidth} height={smallHeight}>
            <Defs>
              <Filter id={filterId} x="-10%" y="-10%" width="120%" height="120%">
                <FeGaussianBlur stdDeviation={blur / k} />
              </Filter>
            </Defs>
            <SvgImage
              x={0}
              y={0}
              width={smallWidth}
              height={smallHeight}
              href={{ uri: photo.uri }}
              preserveAspectRatio="xMidYMid slice"
              filter={`url(#${filterId})`}
            />
          </Svg>
        </View>
      </View>
    );
  }

  // Scenes are vector, so the blur radius is expressed in scene units: divide the
  // wanted dp sigma by the scale "slice" applies when it fits the scene to the box.
  // (Shrinking the SVG by k shrinks that scale by k too, so the ratio is unchanged.)
  const scale = Math.max(width / SCENE_WIDTH, height / SCENE_HEIGHT);
  return (
    <View style={frame}>
      <View style={scaledUp}>
        <Svg
          width={smallWidth}
          height={smallHeight}
          viewBox={`0 0 ${SCENE_WIDTH} ${SCENE_HEIGHT}`}
          preserveAspectRatio="xMidYMid slice"
        >
          <Defs>
            <Filter id={filterId} x="-10%" y="-10%" width="120%" height="120%">
              <FeGaussianBlur stdDeviation={blur / scale} />
            </Filter>
          </Defs>
          <G filter={`url(#${filterId})`}>{renderScene(photo.id)}</G>
        </Svg>
      </View>
    </View>
  );
});

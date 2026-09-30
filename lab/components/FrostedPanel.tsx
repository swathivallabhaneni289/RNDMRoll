// Retired on 2026-09-30 after the developer preferred captions below the photo; nothing imports this file, and it is kept for reference only.
import { ReactNode, useState } from 'react';
import { LayoutChangeEvent, StyleSheet, View } from 'react-native';
import { frost, FrostVariant } from '@/lab/theme/frost';
import type { PhotoRef } from '@/lab/data/photos';
import { LabPhoto } from './LabPhoto';

type Props = {
  variant: FrostVariant;
  /** The photo this panel sits on, and the size that photo is drawn at. */
  photo: PhotoRef;
  photoWidth: number;
  photoHeight: number;
  children: ReactNode;
};

/**
 * The one frosted surface in the lab. It is anchored to the bottom of a photo
 * container and is made of three stacked layers, nothing more:
 *
 *   1. a blurred copy of the same photo, offset so it lines up with the sharp
 *      photo underneath (this is what makes it read as glass rather than a tint),
 *   2. a warm-white tint from the design tokens,
 *   3. a hairline rim.
 *
 * The parent must be the photo container (position relative, overflow hidden) and
 * the panel is laid out by its own content height, so the blurred copy is placed
 * from the panel's measured position. The panel stays invisible for the single
 * frame before that measurement lands.
 */
export function FrostedPanel({ variant, photo, photoWidth, photoHeight, children }: Props) {
  const spec = frost[variant];
  const [top, setTop] = useState<number | null>(null);

  function handleLayout(event: LayoutChangeEvent) {
    setTop(event.nativeEvent.layout.y);
  }

  return (
    <View
      onLayout={handleLayout}
      style={{
        position: 'absolute',
        left: spec.inset,
        right: spec.inset,
        bottom: spec.inset,
        borderRadius: spec.radius,
        overflow: 'hidden',
        opacity: top === null ? 0 : 1,
      }}
    >
      {top !== null ? (
        <View
          accessibilityElementsHidden
          importantForAccessibility="no-hide-descendants"
          style={{
            position: 'absolute',
            left: -spec.inset,
            top: -top,
            width: photoWidth,
            height: photoHeight,
            pointerEvents: 'none',
          }}
        >
          <LabPhoto photo={photo} blur={spec.blur} width={photoWidth} height={photoHeight} />
        </View>
      ) : null}
      <View style={[StyleSheet.absoluteFill, { backgroundColor: spec.tint, pointerEvents: 'none' }]} />
      <View style={{ padding: spec.padding }}>{children}</View>
      <View
        style={[
          StyleSheet.absoluteFill,
          {
            borderRadius: spec.radius,
            borderWidth: StyleSheet.hairlineWidth,
            borderColor: spec.rim,
            pointerEvents: 'none',
          },
        ]}
      />
    </View>
  );
}

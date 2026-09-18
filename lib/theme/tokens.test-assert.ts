/**
 * Compile-time contract pins for lib/theme/tokens.ts.
 *
 * This file is never imported at runtime — nothing in the app imports it. It exists
 * purely so `tsc --noEmit` fails loudly if a later plan edits a token value that
 * another already-planned screen relies on. Every binding below is `export const`
 * using a `satisfies` expression (never a bare local `const`): Task 1 sets
 * `"strict": true`, and if `expo/tsconfig.base` enables `noUnusedLocals` under it,
 * an unused local binding would fail the very typecheck this file exists to
 * strengthen. Exporting keeps each binding used from the compiler's point of view.
 *
 * If an assertion below fails to compile, a later plan changed a token another
 * plan already depends on — resolve the conflict deliberately, do not silently
 * edit this file to match the new value.
 */

import { color, space, minTouchTarget, radius, type, elevation, texture, tokens } from './tokens';

export const pinColorDominant = color.dominant satisfies '#F6F5F2';
export const pinColorSecondary = color.secondary satisfies '#E8E7E3';
export const pinColorCard = color.card satisfies '#FFFFFF';
export const pinColorInk = color.ink satisfies '#111111';
export const pinColorMuted = color.muted satisfies '#625F5B';
export const pinColorDivider = color.divider satisfies '#D3D0C9';
export const pinColorDestructive = color.destructive satisfies '#9A3B32';
export const pinColorSuccess = color.success satisfies '#416B4C';

export const pinSpaceMd = space.md satisfies 16;
export const pinSpaceXxxl = space.xxxl satisfies 64;
export const pinMinTouchTarget = minTouchTarget satisfies 44;

export const pinRadiusMd = radius.md satisfies 8;
export const pinRadiusLg = radius.lg satisfies 24;
export const pinRadiusFull = radius.full satisfies 9999;

export const pinBodyFontSize = type.body.fontSize satisfies 16;
export const pinButtonFontFamily = type.button.fontFamily satisfies 'WorkSans_600SemiBold';
export const pinHeadingFontFamily = type.heading.fontFamily satisfies 'Domine_600SemiBold';
export const pinDisplayFontSize = type.display.fontSize satisfies 40;
export const pinDisplayFontFamily = type.display.fontFamily satisfies 'Domine_600SemiBold';

export const pinElevationRaisedShadowOpacity = elevation.raised.shadowOpacity satisfies 0.18;
export const pinElevationCardShadowRadius = elevation.card.shadowRadius satisfies 6;
export const pinElevationSubtleElevation = elevation.subtle.elevation satisfies 0;

export const pinTextureDotColor = texture.dotColor satisfies 'rgba(17,17,17,0.08)';
export const pinTextureSpacing = texture.spacing satisfies 16;

export const pinTokensShape = tokens satisfies {
  color: typeof color;
  space: typeof space;
  minTouchTarget: typeof minTouchTarget;
  radius: typeof radius;
  type: typeof type;
  elevation: typeof elevation;
  texture: typeof texture;
};

/**
 * UI-SPEC design tokens (revision 8's values; revision 9 carries every value in this
 * file forward unchanged — see 01-UI-SPEC.md revision_reason). Every color, spacing,
 * radius, typography, and elevation value used anywhere in this app must come from
 * this module. No screen writes a raw hex string.
 */

export const color = {
  dominant: '#F6F5F2',
  secondary: '#E8E7E3',
  card: '#FFFFFF',
  ink: '#111111',
  muted: '#625F5B',
  divider: '#D3D0C9',
  destructive: '#9A3B32',
  success: '#416B4C',
  /** Unfocused input border and unselected sign-in method outline. */
  inkRest: 'rgba(17,17,17,0.15)',
  /** Inactive step dots and icon-badge border. */
  inkInactive: 'rgba(17,17,17,0.2)',
  /** Disabled CTA fill. */
  inkDisabled: 'rgba(17,17,17,0.35)',
  /** Avatar placeholder silhouette. */
  inkAvatarPlaceholder: 'rgba(17,17,17,0.4)',
  /** Background dot-grid texture dots. */
  inkTexture: 'rgba(17,17,17,0.08)',
  /** Disabled CTA label, rendered on top of the disabled Ink fill above. */
  onInkDisabled: 'rgba(246,245,242,0.7)',
} as const;

export const space = {
  xs: 4,
  sm: 8,
  md: 16,
  lg: 24,
  xl: 32,
  xxl: 48,
  xxxl: 64,
} as const;

/**
 * Minimum tappable hit-area floor (applied via padding or `hitSlop`), for icon-only
 * controls and tappable rows. This is a hit-area floor, not a spacing step — do not
 * treat it as part of the `space` scale.
 */
export const minTouchTarget = 44;

export const radius = {
  sm: 4,
  /**
   * The hard maximum radius for anything tappable (buttons, inputs, chips). No
   * tappable element may use a larger radius than this in this phase.
   */
  md: 8,
  /**
   * Named exception to the `md` cap, reserved for large, non-interactive content
   * containers only: the profile-view container card and the log-out confirmation
   * sheet. Never applied to anything tappable.
   */
  lg: 24,
  /**
   * Exists solely for the circular avatar and icon-badge tokens. Must never be
   * applied to a button, input, or chip.
   */
  full: 9999,
} as const;

export const type = {
  body: { fontSize: 16, lineHeight: 24, fontFamily: 'WorkSans_400Regular' },
  label: { fontSize: 14, lineHeight: 20, fontFamily: 'WorkSans_400Regular' },
  button: { fontSize: 16, lineHeight: 24, fontFamily: 'WorkSans_600SemiBold' },
  heading: { fontSize: 22, lineHeight: 28, fontFamily: 'Domine_600SemiBold' },
  display: { fontSize: 40, lineHeight: 46, fontFamily: 'Domine_600SemiBold' },
} as const;

export const elevation = {
  /** Secondary-surface panels at rest: intentionally flat/recessed, no shadow keys at all. */
  subtle: {
    backgroundColor: color.secondary,
    elevation: 0,
  },
  card: {
    backgroundColor: color.card,
    shadowColor: color.ink,
    shadowOffset: { width: 0, height: 2 },
    shadowOpacity: 0.08,
    shadowRadius: 6,
    elevation: 3,
  },
  raised: {
    backgroundColor: color.ink,
    shadowColor: color.ink,
    shadowOffset: { width: 0, height: 4 },
    shadowOpacity: 0.18,
    shadowRadius: 10,
    elevation: 6,
  },
} as const;

export const texture = {
  dotDiameter: 2,
  dotColor: color.inkTexture,
  spacing: 16,
} as const;

export const tokens = {
  color,
  space,
  minTouchTarget,
  radius,
  type,
  elevation,
  texture,
} as const;

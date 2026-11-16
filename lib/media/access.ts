/**
 * The words of the alert shown when camera or photo access is turned off for the app (iOS asks
 * only once, so every later tap is refused the same way). No imports: scripts/check-forms.ts runs
 * this file. The alert itself is showAccessOff in lib/media/showAccessOff.ts.
 */

/** "Camera access is off" for the camera, "Photo access is off" for the photo library. */
export function accessOffTitle(source: 'camera' | 'library'): string {
  return source === 'camera' ? 'Camera access is off' : 'Photo access is off';
}

export const ACCESS_OFF_MESSAGE = 'Turn it on in Settings.';

/** Closes the alert. */
export const ACCESS_OFF_NOT_NOW = 'Not now';

/** Opens the app's page in the iPhone's Settings app. */
export const ACCESS_OFF_OPEN_SETTINGS = 'Open Settings';

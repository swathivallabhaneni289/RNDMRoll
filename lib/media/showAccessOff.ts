import { Alert, Linking } from 'react-native';
import {
  ACCESS_OFF_MESSAGE,
  ACCESS_OFF_NOT_NOW,
  ACCESS_OFF_OPEN_SETTINGS,
  accessOffTitle,
} from '@/lib/media/access';

/**
 * Says that camera or photo access is off, in the phone's own alert: "Photo access is off" or
 * "Camera access is off", "Turn it on in Settings.", with Not now and Open Settings. Open Settings
 * goes to the app's page in the Settings app. Call it when a permission request comes back not
 * granted.
 */
export function showAccessOff(source: 'camera' | 'library'): void {
  Alert.alert(accessOffTitle(source), ACCESS_OFF_MESSAGE, [
    { text: ACCESS_OFF_NOT_NOW, style: 'cancel' },
    {
      text: ACCESS_OFF_OPEN_SETTINGS,
      onPress: () => {
        Linking.openSettings().catch(() => undefined);
      },
    },
  ]);
}

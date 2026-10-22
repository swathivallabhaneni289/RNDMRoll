import { useEffect } from 'react';
import { Alert } from 'react-native';
import { useSession } from '@/lib/session/store';

/**
 * The Log out confirmation of Settings: the phone's own alert, the one every app shows, with
 * Cancel and a red Log out. 'Log out' ends the session and the root guard then returns to
 * Welcome. Shown whenever `visible` turns true; `onClose` runs when the alert goes away.
 */
export function LogOutSheet({ visible, onClose }: { visible: boolean; onClose: () => void }) {
  const { signOut } = useSession();

  useEffect(() => {
    if (!visible) return;
    Alert.alert(
      'Log out',
      'Are you sure you want to log out?',
      [
        { text: 'Cancel', style: 'cancel', onPress: onClose },
        {
          text: 'Log out',
          style: 'destructive',
          onPress: () => {
            onClose();
            // A log out that fails (the keychain could not be cleared) leaves the person signed
            // in, as before the tap; Log out can be tapped again.
            signOut().catch(() => undefined);
          },
        },
      ],
      { cancelable: true, onDismiss: onClose },
    );
    // Only a change of `visible` shows the alert again.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [visible]);

  return null;
}

import { useState } from 'react';
import { ConfirmSheet } from '@/components/ui/ConfirmSheet';
import { useSession } from '@/lib/session/store';

/**
 * The Log out sheet of Settings: the words of the profile form's sheet (the developer's, from
 * Phase 1) in the shared sheet with one row (an are-you-sure bar; tap outside it to cancel), so no new rounded shape exists. The built sheet has no
 * per-row red flag yet, so 'Log out' is plain ink here; the red Log out is the Settings row.
 * 'Log out' ends the session and the root guard then returns to Welcome.
 */
export function LogOutSheet({ visible, onClose }: { visible: boolean; onClose: () => void }) {
  const { signOut } = useSession();
  const [busy, setBusy] = useState(false);

  async function confirm() {
    if (busy) return;
    setBusy(true);
    try {
      await signOut();
    } finally {
      setBusy(false);
      onClose();
    }
  }

  return (
    <ConfirmSheet
      visible={visible}
      title="Are you sure you want to log out?"
      confirmLabel="Log out"
      onConfirm={confirm}
      onDismiss={onClose}
      busy={busy}
    />
  );
}

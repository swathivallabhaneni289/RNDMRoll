import { useState } from 'react';
import { Modal, Pressable, View } from 'react-native';
import { useRouter } from 'expo-router';
import { Image } from 'expo-image';
import { Ionicons } from '@expo/vector-icons';
import { AppText } from '@/components/ui/AppText';
import { IconBadge } from '@/components/ui/IconBadge';
import { Screen } from '@/components/ui/Screen';
import { TextButton } from '@/components/ui/TextButton';
import { color, elevation, radius, space } from '@/lib/theme/tokens';
import { useSession } from '@/lib/session/store';

const AVATAR_DIAMETER = 96;
const AVATAR_GLYPH_SIZE = 44;

/**
 * ACCT-03's profile view. D-07 is exact about scope: identity fields only
 * (avatar, name, username, bio), nothing summarizing a feature that
 * doesn't exist yet this phase, since a placeholder for it would be
 * inventing product surface with no data behind it. Renders the session
 * user directly (lib/session/store.ts); edit.tsx's reloadUser() call after
 * a save is what keeps this screen fresh, so it performs no fetch of its
 * own.
 */
export default function ProfileScreen() {
  const router = useRouter();
  const { user, signOut } = useSession();
  const [confirmingSignOut, setConfirmingSignOut] = useState(false);
  const [signingOut, setSigningOut] = useState(false);

  const hasBio = Boolean(user?.bio && user.bio.length > 0);

  async function handleConfirmSignOut() {
    setSigningOut(true);
    try {
      await signOut();
      // The root guard (plan 01-05) routes back to the method chooser once
      // status flips to unauthenticated; this screen performs no explicit
      // navigation of its own.
    } finally {
      setSigningOut(false);
      setConfirmingSignOut(false);
    }
  }

  return (
    <Screen>
      <View style={{ paddingTop: space.xl, paddingBottom: space.xl }}>
        <AppText role="heading">Profile</AppText>

        <View
          style={{
            marginTop: space.lg,
            borderRadius: radius.lg,
            padding: space.lg,
            ...elevation.card,
          }}
        >
          <View style={{ alignItems: 'center' }}>
            <View
              style={{
                width: AVATAR_DIAMETER,
                height: AVATAR_DIAMETER,
                borderRadius: radius.full,
                backgroundColor: color.secondary,
                alignItems: 'center',
                justifyContent: 'center',
                overflow: 'hidden',
              }}
            >
              {user?.avatar_url ? (
                <Image
                  source={{ uri: user.avatar_url }}
                  style={{ width: AVATAR_DIAMETER, height: AVATAR_DIAMETER }}
                  contentFit="cover"
                  accessibilityLabel="Profile photo"
                />
              ) : (
                <Ionicons name="person" size={AVATAR_GLYPH_SIZE} color={color.inkAvatarPlaceholder} />
              )}
            </View>

            <View style={{ marginTop: space.md, alignItems: 'center' }}>
              <AppText role="heading">{user?.name ?? ''}</AppText>
              <View style={{ marginTop: space.xs }}>
                <AppText role="body" tone="muted">{`@${user?.username ?? ''}`}</AppText>
              </View>
            </View>
          </View>

          <View style={{ marginTop: space.lg }}>
            {hasBio ? (
              <AppText role="body">{user?.bio}</AppText>
            ) : (
              <Pressable
                onPress={() => router.push('/(app)/profile/edit')}
                accessibilityRole="button"
                accessibilityLabel="Add a bio"
                style={{ flexDirection: 'row', alignItems: 'center' }}
              >
                <View accessibilityElementsHidden importantForAccessibility="no-hide-descendants">
                  <IconBadge name="create-outline" surface="secondary" accessibilityLabel="Add a bio" />
                </View>
                <View style={{ marginLeft: space.md, flex: 1 }}>
                  <AppText role="heading">Add a bio</AppText>
                  <AppText role="body" tone="muted">
                    Tell people what you're rolling for.
                  </AppText>
                </View>
              </Pressable>
            )}
          </View>
        </View>

        <View style={{ marginTop: space.xl, alignItems: 'center' }}>
          <TextButton label="Edit profile" onPress={() => router.push('/(app)/profile/edit')} />
        </View>

        <View style={{ marginTop: space.md, alignItems: 'center' }}>
          <TextButton
            label="Log out"
            tone="destructive"
            onPress={() => setConfirmingSignOut(true)}
          />
        </View>
      </View>

      <Modal
        visible={confirmingSignOut}
        transparent
        animationType="fade"
        onRequestClose={() => setConfirmingSignOut(false)}
      >
        <View style={{ flex: 1, backgroundColor: color.scrimOverlay, justifyContent: 'flex-end' }}>
          <View
            style={{
              backgroundColor: color.card,
              borderTopLeftRadius: radius.lg,
              borderTopRightRadius: radius.lg,
              padding: space.lg,
              paddingBottom: space.xl,
            }}
          >
            <AppText role="body">Log out of RNDMRoll? You'll need to sign back in.</AppText>
            <View style={{ marginTop: space.lg, alignItems: 'center' }}>
              <TextButton
                label="Log out"
                tone="destructive"
                disabled={signingOut}
                onPress={handleConfirmSignOut}
              />
            </View>
            <View style={{ marginTop: space.md, alignItems: 'center' }}>
              <TextButton
                label="Stay logged in"
                disabled={signingOut}
                onPress={() => setConfirmingSignOut(false)}
              />
            </View>
          </View>
        </View>
      </Modal>
    </Screen>
  );
}

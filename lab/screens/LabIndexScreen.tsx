import { Pressable, StyleSheet, View } from 'react-native';
import { useRouter } from 'expo-router';
import * as ImagePicker from 'expo-image-picker';
import { Ionicons } from '@expo/vector-icons';
import { Screen } from '@/components/ui/Screen';
import { TextButton } from '@/components/ui/TextButton';
import { color, space } from '@/lib/theme/tokens';
import { FEED } from '@/lab/data/entries';
import { setUserPhotos, useUserPhotos } from '@/lab/data/photos';
import { LabText } from '@/lab/components/LabText';

function MenuRow({
  title,
  hint,
  onPress,
  testID,
}: {
  title: string;
  hint: string;
  onPress: () => void;
  testID: string;
}) {
  return (
    <Pressable
      onPress={onPress}
      accessibilityRole="button"
      testID={testID}
      style={{
        minHeight: 64,
        paddingVertical: space.md,
        flexDirection: 'row',
        alignItems: 'center',
        borderBottomWidth: StyleSheet.hairlineWidth,
        borderBottomColor: color.divider,
      }}
    >
      <View style={{ flex: 1 }}>
        <LabText role="heading">{title}</LabText>
        <LabText role="label" tone="muted">
          {hint}
        </LabText>
      </View>
      <Ionicons name="chevron-forward" size={20} color={color.ink} />
    </Pressable>
  );
}

/**
 * Development menu for the lab. Not part of the product:
 * the whole lab folder is behind __DEV__ and reached by deep link.
 */
export default function LabIndexScreen() {
  const router = useRouter();
  const photos = useUserPhotos();

  async function pickPhotos() {
    const result = await ImagePicker.launchImageLibraryAsync({
      mediaTypes: ['images'],
      allowsMultipleSelection: true,
      selectionLimit: 12,
      quality: 0.8,
    });
    if (!result.canceled && result.assets.length > 0) {
      setUserPhotos(result.assets.map((asset) => asset.uri));
    }
  }

  return (
    <Screen scroll={false}>
      <View style={{ paddingTop: space.xl }}>
        <LabText role="display">Lab</LabText>
        <LabText tone="muted" style={{ marginTop: space.sm }}>
          Three screens: feed, diary and entry detail. Development builds only.
        </LabText>
      </View>

      <View style={{ marginTop: space.xl }}>
        <MenuRow
          title="Friend feed"
          hint="Caption and comments below each photo"
          onPress={() => router.push('/lab/feed')}
          testID="lab-feed"
        />
        <MenuRow
          title="Personal diary"
          hint="Grid and diary views"
          onPress={() => router.push('/lab/diary')}
          testID="lab-diary"
        />
        <MenuRow
          title="Entry detail"
          hint="Opened without a source photo"
          onPress={() => router.push({ pathname: '/lab/entry/[id]', params: { id: FEED[0].id, from: 'feed' } })}
          testID="lab-entry"
        />
      </View>

      <View style={{ marginTop: space.xl }}>
        <LabText role="label" tone="muted" uppercase tracking={1.2}>
          Photos
        </LabText>
        <LabText style={{ marginTop: space.xs }}>
          {photos.length > 0 ? `${photos.length} from your library` : 'Designed placeholders'}
        </LabText>
        <View style={{ marginTop: space.md, alignItems: 'flex-start', gap: space.md }}>
          <TextButton label="Use photos from my library" onPress={pickPhotos} />
          {photos.length > 0 ? <TextButton label="Back to placeholders" tone="muted" onPress={() => setUserPhotos([])} /> : null}
        </View>
      </View>
    </Screen>
  );
}

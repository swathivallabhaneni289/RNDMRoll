import { View } from 'react-native';
import { useRouter } from 'expo-router';
import { TextButton } from '@/components/ui/TextButton';

/** Dev-only way back to the lab menu; the lab has no tab bar because the product has none yet. */
export function LabBackLink({ label = 'Lab' }: { label?: string }) {
  const router = useRouter();
  return (
    <View style={{ alignSelf: 'flex-start' }}>
      <TextButton
        label={label}
        tone="muted"
        onPress={() => (router.canGoBack() ? router.back() : router.replace('/lab'))}
      />
    </View>
  );
}

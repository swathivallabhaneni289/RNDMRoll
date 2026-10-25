import { View } from 'react-native';
import type { Href } from 'expo-router';
import type { Edge } from 'react-native-safe-area-context';
import { BackRow } from '@/components/settings/BackRow';
import { AppText } from '@/components/ui/AppText';
import { Screen } from '@/components/ui/Screen';
import type { LegalBlock, LegalDocument } from '@/lib/legal/documents';
import { space } from '@/lib/theme/tokens';

/**
 * The Terms of Use or the Privacy Policy, read in the app: Back, the title, the date the text took
 * effect, the opening lines, then each section's heading with its paragraphs and lists. The words
 * live in lib/legal/documents.ts, which also writes the copies in docs/legal (scripts/check-legal.ts),
 * so the app and GitHub always say the same thing.
 *
 * Opened from the sign-up page (signed out, all four safe-area edges) and from Settings (signed in,
 * where the bottom bar adds the bottom inset itself, so the route passes the other three edges).
 */
export function LegalScreen({
  document,
  fallback,
  edges,
}: {
  document: LegalDocument;
  /** Where Back lands when the page was opened with nothing behind it. */
  fallback: Href;
  edges?: readonly Edge[];
}) {
  return (
    <Screen edges={edges}>
      <View style={{ paddingTop: space.lg, paddingBottom: space.xl }}>
        <BackRow fallback={fallback} />

        <View style={{ marginTop: space.md }}>
          <AppText role="title" accessibilityRole="header">
            {document.title}
          </AppText>
        </View>
        <View style={{ marginTop: space.sm }}>
          <AppText role="label" tone="muted">
            {`Effective ${document.effective}`}
          </AppText>
        </View>

        {document.intro.map((text) => (
          <View key={text} style={{ marginTop: space.md }}>
            <AppText role="body">{text}</AppText>
          </View>
        ))}

        {document.sections.map((section) => (
          <View key={section.heading} style={{ marginTop: space.xl }}>
            <AppText role="heading" accessibilityRole="header">
              {section.heading}
            </AppText>
            {section.blocks.map((block, index) => (
              <Block key={index} block={block} />
            ))}
          </View>
        ))}
      </View>
    </Screen>
  );
}

/** One paragraph, or one list with a plain bullet before each item. */
function Block({ block }: { block: LegalBlock }) {
  if (block.kind === 'paragraph') {
    return (
      <View style={{ marginTop: space.sm }}>
        <AppText role="body">{block.text}</AppText>
      </View>
    );
  }
  return (
    <View style={{ marginTop: space.sm }}>
      {block.items.map((item, index) => (
        <View key={item} style={{ flexDirection: 'row', marginTop: index === 0 ? 0 : space.xs }}>
          {/* The bullet is drawn, not read: VoiceOver reads each item as its own line. */}
          <View accessibilityElementsHidden importantForAccessibility="no-hide-descendants" style={{ width: space.md }}>
            <AppText role="body">{'•'}</AppText>
          </View>
          <View style={{ flex: 1 }}>
            <AppText role="body">{item}</AppText>
          </View>
        </View>
      ))}
    </View>
  );
}

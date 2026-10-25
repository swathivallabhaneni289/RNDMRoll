import type { LegalDocument } from './documents';

/**
 * A legal document as the Markdown copy kept in docs/legal for GitHub. scripts/check-legal.ts
 * writes the copies and fails when one no longer matches, so the words are edited only in
 * lib/legal/documents.ts. This file imports only a type, so Node loads it without the app.
 */
export function legalMarkdown(document: LegalDocument): string {
  const lines: string[] = [
    '<!-- Written by scripts/check-legal.ts from lib/legal/documents.ts: edit that file, then run the script with --write. -->',
    '',
    `# ${document.title}`,
    '',
    `Effective ${document.effective}`,
  ];
  for (const text of document.intro) lines.push('', text);
  for (const section of document.sections) {
    lines.push('', `## ${section.heading}`);
    for (const block of section.blocks) {
      if (block.kind === 'paragraph') lines.push('', block.text);
      else lines.push('', ...block.items.map((item) => `- ${item}`));
    }
  }
  return `${lines.join('\n')}\n`;
}

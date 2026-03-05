import { useMemo } from 'react';

import { CatalogEntry, DisplayNamePreset } from '../types';

export function resolveDisplayName(
  entry: CatalogEntry | undefined,
  preset: DisplayNamePreset,
  pattern: string,
  context?: { column?: string; aggregation?: string; assetPath?: string; connection?: string }
): string {
  if (!entry) {
    return context?.column ?? 'unknown';
  }

  switch (preset) {
    case 'tagLevel1':
      return entry.metadata?.tagLevel1 || entry.name;
    case 'descriptionEn':
      return entry.metadata?.description?.['en-US'] || entry.name;
    case 'descriptionDe':
      return entry.metadata?.description?.['de-DE'] || entry.name;
    case 'assetPath':
      return context?.assetPath || entry.name;
    case 'custom':
      return resolvePattern(pattern, entry, context);
    default:
      return entry.name;
  }
}

function resolvePattern(
  pattern: string,
  entry: CatalogEntry,
  context?: { column?: string; aggregation?: string; assetPath?: string; connection?: string }
): string {
  if (!pattern) {
    return entry.name;
  }

  let result = pattern;
  result = result.replace(/\{name\}/g, entry.name);
  result = result.replace(/\{tagLevel1\}/g, entry.metadata?.tagLevel1 ?? '');
  result = result.replace(/\{unit\}/g, entry.metadata?.unit ?? '');
  result = result.replace(/\{column\}/g, context?.column ?? '');
  result = result.replace(/\{aggregation\}/g, context?.aggregation ?? '');
  result = result.replace(/\{asset\.path\}/g, context?.assetPath ?? '');
  result = result.replace(/\{connection\}/g, context?.connection ?? '');

  if (entry.metadata?.description) {
    for (const [locale, desc] of Object.entries(entry.metadata.description)) {
      result = result.replace(new RegExp(`\\{description\\.${locale.replace('-', '\\-')}\\}`, 'g'), desc);
    }
  }

  if (entry.labels.length > 0) {
    result = result.replace(/\{label\}/g, entry.labels[0]);
  }

  return result;
}

export function useDisplayNamePreview(
  entries: CatalogEntry[],
  preset: DisplayNamePreset,
  pattern: string
): string[] {
  return useMemo(() => {
    return entries.map((entry) => resolveDisplayName(entry, preset, pattern));
  }, [entries, preset, pattern]);
}

import { useMemo } from 'react';

import { CatalogEntry, DisplayNamePreset } from '../types';

type DisplayNameContext = { column?: string; aggregation?: string; assetPath?: string; connection?: string; locale?: string };

function languageOf(locale: string): string {
  return locale.split('-')[0].toLowerCase();
}

function firstForLanguage(descriptions: Record<string, string>, language: string): string | undefined {
  if (!language) {
    return undefined;
  }
  return Object.keys(descriptions)
    .sort()
    .map((locale) => (languageOf(locale) === language ? descriptions[locale] : undefined))
    .find((description) => !!description);
}

/** Catalog description in the viewer's language: exact locale, same language, English, then the entry name. */
export function localizedDescription(entry: CatalogEntry, locale?: string): string {
  const descriptions = entry.metadata?.description ?? {};
  return (
    (locale ? descriptions[locale] : undefined) ||
    firstForLanguage(descriptions, languageOf(locale ?? '')) ||
    descriptions['en-US'] ||
    firstForLanguage(descriptions, 'en') ||
    entry.name
  );
}

export function resolveDisplayName(
  entry: CatalogEntry | undefined,
  preset: DisplayNamePreset,
  pattern: string,
  context?: DisplayNameContext
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
    case 'description':
      return localizedDescription(entry, context?.locale);
    case 'assetPath':
      return context?.assetPath || entry.name;
    case 'custom':
      return resolvePattern(pattern, entry, context);
    default:
      return entry.name;
  }
}

function resolvePattern(pattern: string, entry: CatalogEntry, context?: DisplayNameContext): string {
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
  if (result.includes('{description}')) {
    result = result.replace(/\{description\}/g, localizedDescription(entry, context?.locale));
  }

  if (entry.metadata?.description) {
    for (const [locale, desc] of Object.entries(entry.metadata.description)) {
      result = result.replace(new RegExp(`\\{description\\.${locale.replace('-', '\\-')}\\}`, 'g'), desc);
    }
  }

  if (entry.labels.length > 0) {
    result = result.replace(/\{label\}/g, entry.labels[0].name);
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

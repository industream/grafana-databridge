import { config } from '@grafana/runtime';

function languageOf(locale: string): string {
  return locale.split('-')[0].toLowerCase();
}

function firstForLanguage(texts: Record<string, string>, language: string): string | undefined {
  if (!language) {
    return undefined;
  }
  return Object.keys(texts)
    .sort()
    .map((locale) => (languageOf(locale) === language ? texts[locale] : undefined))
    .find((text) => !!text);
}

/** Text in the viewer's language: exact locale, same language in another region, then English. */
export function pickLocalized(texts: Record<string, string>, locale?: string): string | undefined {
  return (
    (locale ? texts[locale] : undefined) ||
    firstForLanguage(texts, languageOf(locale ?? '')) ||
    texts['en-US'] ||
    firstForLanguage(texts, 'en')
  );
}

/** Reads translations written as a JSON object or as one "locale=text" pair per line. */
export function parseTranslations(source: string): Record<string, string> {
  const trimmed = source.trim();
  if (trimmed.startsWith('{')) {
    try {
      return JSON.parse(trimmed) as Record<string, string>;
    } catch {
      return {};
    }
  }
  const texts: Record<string, string> = {};
  for (const line of trimmed.split('\n')) {
    const separator = line.indexOf('=');
    if (separator > 0) {
      texts[line.slice(0, separator).trim()] = line.slice(separator + 1).trim();
    }
  }
  return texts;
}

/** The viewer's Grafana language: the resolved UI language, else the user's preference. */
export function viewerLanguage(): string | undefined {
  return config.language || config.bootData?.user?.language || undefined;
}

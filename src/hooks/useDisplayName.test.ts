import { CatalogEntry } from '../types';
import { resolveDisplayName } from './useDisplayName';

function entry(description?: Record<string, string>): CatalogEntry {
  return {
    id: 'id-counter',
    name: 'Parts counter',
    labels: [],
    metadata: description ? { description } : undefined,
  } as unknown as CatalogEntry;
}

describe('resolveDisplayName with the description preset', () => {
  const translated = entry({ 'en-US': 'Parts counter', 'de-DE': 'Stückzähler', 'fr-FR': 'Compteur de pièces' });

  it('shows the description in the viewer language', () => {
    expect(resolveDisplayName(translated, 'description', '', { locale: 'de-DE' })).toBe('Stückzähler');
    expect(resolveDisplayName(translated, 'description', '', { locale: 'fr-FR' })).toBe('Compteur de pièces');
  });

  it('matches the language when the region differs', () => {
    expect(resolveDisplayName(translated, 'description', '', { locale: 'de-AT' })).toBe('Stückzähler');
  });

  it('falls back to English, then to the entry name', () => {
    expect(resolveDisplayName(translated, 'description', '', { locale: 'pt-BR' })).toBe('Parts counter');
    expect(resolveDisplayName(entry({ 'de-DE': 'Stückzähler' }), 'description', '', { locale: 'pt-BR' })).toBe(
      'Parts counter'
    );
    expect(resolveDisplayName(entry(), 'description', '', { locale: 'fr-FR' })).toBe('Parts counter');
  });

  it('fills the {description} placeholder of a custom pattern in the viewer language', () => {
    expect(resolveDisplayName(translated, 'custom', '{description} #1', { locale: 'fr-FR' })).toBe(
      'Compteur de pièces #1'
    );
  });
});

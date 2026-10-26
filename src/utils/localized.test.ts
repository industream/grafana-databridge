import { parseTranslations, pickLocalized } from './localized';

describe('pickLocalized', () => {
  const texts = { 'en-US': 'Line status', 'de-DE': 'Linienstatus', 'fr-FR': 'État de la ligne' };

  it('returns the text of the exact locale', () => {
    expect(pickLocalized(texts, 'de-DE')).toBe('Linienstatus');
  });

  it('matches the language when the region differs', () => {
    expect(pickLocalized(texts, 'fr-BE')).toBe('État de la ligne');
  });

  it('falls back to English, then to any English variant', () => {
    expect(pickLocalized(texts, 'pt-BR')).toBe('Line status');
    expect(pickLocalized({ 'en-GB': 'Line status (GB)', 'de-DE': 'Linienstatus' }, 'pt-BR')).toBe('Line status (GB)');
  });

  it('returns undefined when nothing matches', () => {
    expect(pickLocalized({ 'de-DE': 'Linienstatus' }, 'pt-BR')).toBeUndefined();
    expect(pickLocalized({}, 'de-DE')).toBeUndefined();
  });
});

describe('parseTranslations', () => {
  it('reads one "locale=text" pair per line and ignores blank lines', () => {
    expect(parseTranslations('en-US=Line status\n\nde-DE = Linienstatus\nfr-FR=État = ligne')).toEqual({
      'en-US': 'Line status',
      'de-DE': 'Linienstatus',
      'fr-FR': 'État = ligne',
    });
  });

  it('accepts a JSON object', () => {
    expect(parseTranslations('{"en-US":"Line status","de-DE":"Linienstatus"}')).toEqual({
      'en-US': 'Line status',
      'de-DE': 'Linienstatus',
    });
  });

  it('returns an empty map for text it cannot read', () => {
    expect(parseTranslations('{not json')).toEqual({});
    expect(parseTranslations('no equals sign')).toEqual({});
  });
});

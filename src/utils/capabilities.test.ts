import { ComboboxOption } from '@grafana/ui';

import { ProviderCapabilities } from '../types';
import {
  NOT_SUPPORTED_SUFFIX,
  decorateAggregationOptions,
  foldQueryName,
  guardAggregationChange,
  isAggregationSupported,
} from './capabilities';

const IBAHD: ProviderCapabilities = {
  supportedAggregations: ['min', 'max', 'avg'],
  supportedStats: ['min', 'max', 'mean'],
  supportsExactComputeOnRawWindow: false,
};

const OPTIONS: Array<ComboboxOption<string>> = [
  { label: 'Optimized', value: 'optimized' },
  { label: 'Raw', value: 'none' },
  { label: 'avg', value: 'avg' },
  { label: 'mean', value: 'mean' },
  { label: 'min', value: 'min' },
  { label: 'max', value: 'max' },
  { label: 'sum', value: 'sum' },
  { label: 'stddev', value: 'stddev' },
];

describe('foldQueryName', () => {
  it('folds mean to avg', () => {
    expect(foldQueryName('mean')).toBe('avg');
  });

  it('folds stddev/var synonyms to their canonical name', () => {
    expect(foldQueryName('std')).toBe('stddev');
    expect(foldQueryName('stddev_pop')).toBe('stddev');
    expect(foldQueryName('var_samp')).toBe('var');
    expect(foldQueryName('variance')).toBe('var');
  });

  it('leaves canonical names untouched', () => {
    expect(foldQueryName('min')).toBe('min');
    expect(foldQueryName('sum')).toBe('sum');
  });
});

describe('isAggregationSupported', () => {
  it('always allows optimized and none', () => {
    expect(isAggregationSupported('optimized', IBAHD)).toBe(true);
    expect(isAggregationSupported('none', IBAHD)).toBe(true);
  });

  it('allows aggregations the provider advertises', () => {
    expect(isAggregationSupported('min', IBAHD)).toBe(true);
    expect(isAggregationSupported('max', IBAHD)).toBe(true);
    expect(isAggregationSupported('avg', IBAHD)).toBe(true);
  });

  it('allows mean because it folds to avg', () => {
    expect(isAggregationSupported('mean', IBAHD)).toBe(true);
  });

  it('rejects aggregations the provider cannot compute', () => {
    expect(isAggregationSupported('sum', IBAHD)).toBe(false);
    expect(isAggregationSupported('count', IBAHD)).toBe(false);
    expect(isAggregationSupported('stddev', IBAHD)).toBe(false);
    expect(isAggregationSupported('first', IBAHD)).toBe(false);
  });

  it('degrades open when capabilities are unknown', () => {
    expect(isAggregationSupported('sum', null)).toBe(true);
    expect(isAggregationSupported('stddev', undefined)).toBe(true);
  });
});

describe('decorateAggregationOptions', () => {
  it('marks unsupported options with a description and leaves supported ones untouched', () => {
    const decorated = decorateAggregationOptions(OPTIONS, IBAHD);
    const byValue = Object.fromEntries(decorated.map((o) => [o.value, o]));

    expect(byValue.avg.description).toBeUndefined();
    expect(byValue.mean.description).toBeUndefined();
    expect(byValue.optimized.description).toBeUndefined();
    expect(byValue.sum.description).toBe(NOT_SUPPORTED_SUFFIX);
    expect(byValue.stddev.description).toBe(NOT_SUPPORTED_SUFFIX);
  });

  it('returns options unchanged when capabilities are unknown (degrade open)', () => {
    expect(decorateAggregationOptions(OPTIONS, null)).toBe(OPTIONS);
  });
});

describe('guardAggregationChange', () => {
  it('returns the operation when supported', () => {
    expect(guardAggregationChange('avg', IBAHD)).toBe('avg');
  });

  it('returns undefined for an unsupported operation', () => {
    expect(guardAggregationChange('sum', IBAHD)).toBeUndefined();
  });

  it('returns the operation when capabilities are unknown', () => {
    expect(guardAggregationChange('sum', null)).toBe('sum');
  });
});

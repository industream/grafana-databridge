import { ComboboxOption } from '@grafana/ui';

import { ProviderCapabilities, TagOperation } from '../types';

/**
 * The tooltip/description shown on an aggregation the active provider cannot
 * compute. Kept generic ("active provider") with the provider name appended by
 * the caller when known, e.g. "not supported by IbaHD".
 */
export const NOT_SUPPORTED_SUFFIX = 'not supported by the active provider';

/**
 * Operations that are never provider aggregations and are therefore always
 * enabled: "optimized" (the editor picks a safe per-type aggregation) and
 * "none" (raw, no aggregation at all).
 */
const ALWAYS_ENABLED = new Set<string>(['optimized', 'none']);

/**
 * Fold an option value to the canonical query-time aggregation name so that a
 * provider advertising the canonical form ("stddev", "var", "avg") also enables
 * its synonyms. This is the query-vocabulary fold; the stats vocabulary
 * ("mean") is handled by mapping "mean" → "avg" here too since the per-tag
 * control mixes both labels.
 */
export function foldQueryName(value: string): string {
  switch (value) {
    case 'mean':
      return 'avg';
    case 'std':
    case 'stddev_pop':
    case 'stddev_samp':
      return 'stddev';
    case 'variance':
    case 'var_samp':
    case 'var_pop':
      return 'var';
    default:
      return value;
  }
}

/**
 * Whether an aggregation option is selectable given the provider capabilities.
 *
 * Degrade-open: when `capabilities` is null/undefined (unknown — older image or
 * unreachable DataBridge) everything is supported. "optimized"/"none" are
 * always supported. Otherwise an exact match OR a folded-name match against the
 * advertised set enables the option.
 */
export function isAggregationSupported(
  value: string,
  capabilities: ProviderCapabilities | null | undefined
): boolean {
  if (ALWAYS_ENABLED.has(value)) {
    return true;
  }
  if (!capabilities) {
    return true;
  }
  const supported = capabilities.supportedAggregations;
  return supported.includes(value) || supported.includes(foldQueryName(value));
}

/**
 * Annotate aggregation options with a "not supported" description for the ones
 * the active provider cannot compute. The option stays visible (grey, never
 * hidden) so saved selections remain understandable; selection of an
 * unsupported option must be blocked by the caller's onChange guard.
 *
 * Degrade-open: with unknown capabilities the options are returned unchanged.
 */
export function decorateAggregationOptions(
  options: Array<ComboboxOption<string>>,
  capabilities: ProviderCapabilities | null | undefined
): Array<ComboboxOption<string>> {
  if (!capabilities) {
    return options;
  }
  return options.map((option) => {
    if (isAggregationSupported(option.value, capabilities)) {
      return option;
    }
    return { ...option, description: NOT_SUPPORTED_SUFFIX };
  });
}

/**
 * Guard a tag-operation change against the provider capabilities. Returns the
 * proposed operation when supported, or `undefined` when the user picked an
 * unsupported (greyed) option so the caller can ignore the change without
 * silently rewriting the value.
 */
export function guardAggregationChange(
  operation: TagOperation,
  capabilities: ProviderCapabilities | null | undefined
): TagOperation | undefined {
  return isAggregationSupported(operation, capabilities) ? operation : undefined;
}

import { DataSourceJsonData } from '@grafana/data';
import { DataQuery } from '@grafana/schema';

// --- Query types ---

export type QueryMode = 'dataCatalog' | 'raw';
export type QueryStrategy = 'timeseries' | 'table';

export type AggregationFunction =
  | 'avg'
  | 'min'
  | 'max'
  | 'sum'
  | 'count'
  | 'first'
  | 'last'
  | 'stddev'
  | 'stddev_pop'
  | 'var'
  | 'var_pop';

export type AggregationOrNone = AggregationFunction | 'none';

export type TagOperation = AggregationFunction | 'optimized' | 'none';

export type TimeWindowUnit = 's' | 'm' | 'h' | 'd';

const TIME_WINDOW_UNIT_SECONDS: Record<TimeWindowUnit, number> = {
  s: 1,
  m: 60,
  h: 3600,
  d: 86400,
};

export function timeWindowToSeconds(interval?: number, unit?: TimeWindowUnit): number {
  if (!interval || interval <= 0 || !unit) {
    return 0;
  }
  return interval * TIME_WINDOW_UNIT_SECONDS[unit];
}

export type DisplayNamePreset =
  | 'entryName'
  | 'tagLevel1'
  | 'descriptionEn'
  | 'descriptionDe'
  | 'assetPath'
  | 'custom';

export interface SelectDefinition {
  catalogEntryId?: string;
  column?: string;
  dataType?: string;
  aggregation?: TagOperation;
  alias?: string;
  displayNamePreset?: DisplayNamePreset;
  displayNamePattern?: string;
}

export type ComparisonOperator = 'eq' | 'neq' | 'gt' | 'gte' | 'lt' | 'lte' | 'in' | 'notIn';
export type LogicalOperator = 'and' | 'or';

/**
 * Recursive filter tree: either a logical group (AND/OR with sub-conditions)
 * or a leaf comparison (column op value).
 */
export type FilterDefinition = FilterGroup | FilterCondition;

export interface FilterGroup {
  operator: LogicalOperator;
  conditions: FilterDefinition[];
}

export interface FilterCondition {
  column: string;
  operator: ComparisonOperator;
  value: string | number | boolean;
}

export function isFilterGroup(f: FilterDefinition): f is FilterGroup {
  return 'conditions' in f && Array.isArray((f as FilterGroup).conditions);
}

export interface DataBridgeQuery extends DataQuery {
  mode: QueryMode;
  strategy: QueryStrategy;
  optimizeDisplay: boolean;

  // DataCatalog mode
  catalogEntryIds?: string[];

  // Raw mode
  connectionId?: string;
  databaseName?: string;
  datasetName?: string;

  // SELECT
  select: SelectDefinition[];

  // WHERE
  where?: FilterDefinition;

  // Aggregation / time window
  aggregation?: AggregationFunction;
  timeWindowInterval?: number;
  timeWindowUnit?: TimeWindowUnit;

  // Pagination
  limit?: number;
  offset?: number;
  orderByColumn?: string;
  orderByDirection?: 'asc' | 'desc';

  // Display
  displayNamePreset?: DisplayNamePreset;
  displayNamePattern?: string;
}

export const DEFAULT_QUERY: Partial<DataBridgeQuery> = {
  mode: 'dataCatalog',
  strategy: 'timeseries',
  optimizeDisplay: true,
  select: [],
};

// --- Datasource configuration types ---

export interface DataBridgeOptions extends DataSourceJsonData {
  dataBridgeApiUrl?: string;
  dataCatalogApiUrl?: string;
  sourceConnectionId?: string;

  // Display defaults
  defaultDisplayNamePreset?: DisplayNamePreset;
  defaultAggregation?: AggregationOrNone;

  // Safety limits
  maxRawRows?: number;
  hardLimitRows?: number;
  cacheTtlSeconds?: number;
}

export interface DataBridgeSecureJsonData {
  apiKey?: string;
}

// --- API response types (from CallResource) ---

export interface SourceConnection {
  id: string;
  name: string;
  sourceTypeId: string;
  url: string;
}

export interface DatabaseInfo {
  name: string;
}

export interface DatasetInfo {
  name: string;
}

export interface ColumnInfo {
  name: string;
  type: string;
}

export interface DatasetSchema {
  columns: ColumnInfo[];
}

export interface CatalogEntry {
  id: string;
  name: string;
  sourceConnection?: { id: string; name: string; url: string };
  sourceConnectionId?: string;
  dataType: string;
  labels: Label[];
  metadata: CatalogEntryMetadata | null;
  sourceParams: Record<string, string>;
}

export interface CatalogEntryMetadata {
  tagLevel1?: string;
  description?: Record<string, string>;
  unit?: string;
  min?: number;
  max?: number;
  decimals?: number;
  scale?: number;
}

export interface AssetNode {
  id: string;
  name: string;
  parentId: string | null;
  children: AssetNode[];
  entryIds?: string[];
  entryCount: number;
}

export interface AssetTree {
  id: string;
  name: string;
  nodes: AssetNode[];
}

export interface Label {
  id: string;
  name: string;
}

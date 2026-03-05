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
  aggregation?: AggregationFunction;
  alias?: string;
  displayNamePreset?: DisplayNamePreset;
  displayNamePattern?: string;
}

export interface WhereCondition {
  column: string;
  operator: 'eq' | 'neq' | 'gt' | 'gte' | 'lt' | 'lte' | 'in' | 'notIn';
  value: string | number | boolean;
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
  where?: WhereCondition[];

  // Aggregation / time window
  aggregation?: AggregationFunction;
  timeWindowSeconds?: number;

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
  aggregation: 'avg',
};

// --- Datasource configuration types ---

export interface DataBridgeOptions extends DataSourceJsonData {
  dataBridgeApiUrl?: string;
  dataCatalogApiUrl?: string;

  // Display defaults
  defaultDisplayNamePreset?: DisplayNamePreset;
  defaultAggregation?: AggregationFunction;

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
  dataType: string;
}

export interface DatasetSchema {
  columns: ColumnInfo[];
}

export interface CatalogEntry {
  id: string;
  name: string;
  sourceConnectionId: string;
  dataType: string;
  labels: string[];
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

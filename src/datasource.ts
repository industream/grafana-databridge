import { CoreApp, DataSourceInstanceSettings, MetricFindValue, ScopedVars } from '@grafana/data';
import { DataSourceWithBackend, getTemplateSrv } from '@grafana/runtime';

import {
  DataBridgeQuery,
  DataBridgeOptions,
  DEFAULT_QUERY,
  SourceConnection,
  DatabaseInfo,
  DatasetInfo,
  DatasetSchema,
  CatalogEntry,
  CatalogEntryMetadata,
  AssetTree,
  Label,
} from './types';
import { VariableQuery } from './components/VariableQueryEditor';

export class DataSource extends DataSourceWithBackend<DataBridgeQuery, DataBridgeOptions> {
  readonly settings: DataBridgeOptions;

  constructor(instanceSettings: DataSourceInstanceSettings<DataBridgeOptions>) {
    super(instanceSettings);
    this.settings = instanceSettings.jsonData;
  }

  getDefaultQuery(_: CoreApp): Partial<DataBridgeQuery> {
    const defaultAgg = this.settings.defaultAggregation ?? 'avg';
    const isNone = defaultAgg === 'none';
    return {
      ...DEFAULT_QUERY,
      optimizeDisplay: !isNone,
    };
  }

  applyTemplateVariables(query: DataBridgeQuery, scopedVars: ScopedVars): DataBridgeQuery {
    const templateSrv = getTemplateSrv();
    return {
      ...query,
      connectionId: query.connectionId ? templateSrv.replace(query.connectionId, scopedVars) : undefined,
      databaseName: query.databaseName ? templateSrv.replace(query.databaseName, scopedVars) : undefined,
      datasetName: query.datasetName ? templateSrv.replace(query.datasetName, scopedVars) : undefined,
    };
  }

  async metricFindQuery(query: DataBridgeQuery): Promise<MetricFindValue[]> {
    const vq: VariableQuery = (query as DataBridgeQuery & { variableQuery?: VariableQuery }).variableQuery ?? { type: 'connections' };
    const templateSrv = getTemplateSrv();

    const params: Record<string, string> = { type: vq.type };
    if (vq.connectionId) {
      params.connectionId = templateSrv.replace(vq.connectionId);
    }
    if (vq.databaseName) {
      params.database = templateSrv.replace(vq.databaseName);
    }
    if (vq.label) {
      params.label = templateSrv.replace(vq.label);
    }
    if (vq.search) {
      params.search = templateSrv.replace(vq.search);
    }

    const results = await this.getResource<Array<{ text: string; value: string }>>('variables', params);
    return results.map((r) => ({ text: r.text, value: r.value }));
  }

  filterQuery(query: DataBridgeQuery): boolean {
    if (query.mode === 'dataCatalog') {
      return (query.catalogEntryIds?.length ?? 0) > 0 || (query.select?.length ?? 0) > 0;
    }
    return !!query.connectionId && !!query.databaseName && !!query.datasetName;
  }

  // --- Resource API helpers (CallResource proxied to Go backend) ---

  async getConnections(): Promise<SourceConnection[]> {
    return this.getResource<SourceConnection[]>('connections');
  }

  async getDatabases(connectionId: string): Promise<DatabaseInfo[]> {
    return this.getResource<DatabaseInfo[]>('databases', { connectionId });
  }

  async getDatasets(connectionId: string, databaseName: string): Promise<DatasetInfo[]> {
    return this.getResource<DatasetInfo[]>('datasets', { connectionId, database: databaseName });
  }

  async getSchema(connectionId: string, databaseName: string, datasetName: string): Promise<DatasetSchema> {
    return this.getResource<DatasetSchema>('schema', {
      connectionId,
      database: databaseName,
      dataset: datasetName,
    });
  }

  async getCatalogEntries(params: { ids?: string; label?: string; search?: string }): Promise<CatalogEntry[]> {
    const entries = await this.getResource<CatalogEntry[]>('catalog-entries', params);
    return entries.map(normalizeEntry);
  }

  async getAssetTree(): Promise<AssetTree[]> {
    return this.getResource<AssetTree[]>('asset-tree');
  }

  async getNodeEntries(nodeId: string): Promise<CatalogEntry[]> {
    const entries = await this.getResource<CatalogEntry[]>('node-entries', { nodeId });
    return entries.map(normalizeEntry);
  }

  async getLabels(): Promise<Label[]> {
    return this.getResource<Label[]>('labels');
  }

  async clearCache(): Promise<void> {
    await this.postResource('cache/clear', {});
  }
}

/**
 * Normalize metadata from DataCatalog API:
 * - description: parse JSON string into Record<string, string>
 * - min/max/decimals/scale: convert string to number
 */
function normalizeEntry(entry: CatalogEntry): CatalogEntry {
  if (!entry.metadata) {
    return entry;
  }

  const meta = { ...entry.metadata } as Record<string, unknown>;

  // Parse description if it's a JSON string
  if (typeof meta.description === 'string') {
    try {
      meta.description = JSON.parse(meta.description as string);
    } catch {
      // Leave as-is if not valid JSON
    }
  }

  // Convert numeric fields from string to number
  for (const key of ['min', 'max', 'decimals', 'scale'] as const) {
    if (typeof meta[key] === 'string' && meta[key] !== '') {
      const num = Number(meta[key]);
      if (!isNaN(num)) {
        meta[key] = num;
      }
    }
  }

  return { ...entry, metadata: meta as CatalogEntryMetadata };
}

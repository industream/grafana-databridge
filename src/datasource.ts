import { CoreApp, DataSourceInstanceSettings, ScopedVars } from '@grafana/data';
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
  AssetTree,
  Label,
} from './types';

export class DataSource extends DataSourceWithBackend<DataBridgeQuery, DataBridgeOptions> {
  constructor(instanceSettings: DataSourceInstanceSettings<DataBridgeOptions>) {
    super(instanceSettings);
  }

  getDefaultQuery(_: CoreApp): Partial<DataBridgeQuery> {
    return DEFAULT_QUERY;
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
    return this.getResource<CatalogEntry[]>('catalog-entries', params);
  }

  async getAssetTree(): Promise<AssetTree[]> {
    return this.getResource<AssetTree[]>('asset-tree');
  }

  async getNodeEntries(nodeId: string): Promise<CatalogEntry[]> {
    return this.getResource<CatalogEntry[]>('node-entries', { nodeId });
  }

  async getLabels(): Promise<Label[]> {
    return this.getResource<Label[]>('labels');
  }
}

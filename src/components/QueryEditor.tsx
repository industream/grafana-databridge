import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { Combobox, InlineField, InlineFieldRow, RadioButtonGroup, Switch } from '@grafana/ui';
import { QueryEditorProps, SelectableValue } from '@grafana/data';

import { DataSource } from '../datasource';
import {
  DataBridgeOptions,
  DataBridgeQuery,
  QueryMode,
  QueryStrategy,
  AggregationFunction,
  SourceConnection,
  DatabaseInfo,
  DatasetInfo,
} from '../types';

type Props = QueryEditorProps<DataSource, DataBridgeQuery, DataBridgeOptions>;

const MODE_OPTIONS: Array<SelectableValue<QueryMode>> = [
  { label: 'DataCatalog', value: 'dataCatalog' },
  { label: 'Raw', value: 'raw' },
];

const STRATEGY_OPTIONS: Array<SelectableValue<QueryStrategy>> = [
  { label: 'Time Series', value: 'timeseries' },
  { label: 'Table', value: 'table' },
];

const AGGREGATION_OPTIONS = [
  { label: 'Average', value: 'avg' as const },
  { label: 'Minimum', value: 'min' as const },
  { label: 'Maximum', value: 'max' as const },
  { label: 'Sum', value: 'sum' as const },
  { label: 'Count', value: 'count' as const },
  { label: 'First', value: 'first' as const },
  { label: 'Last', value: 'last' as const },
];

const LABEL_WIDTH = 16;

export function QueryEditor({ query, onChange, onRunQuery, datasource }: Props) {
  const [connections, setConnections] = useState<SourceConnection[]>([]);
  const [databases, setDatabases] = useState<DatabaseInfo[]>([]);
  const [datasets, setDatasets] = useState<DatasetInfo[]>([]);

  const updateQuery = useCallback(
    (patch: Partial<DataBridgeQuery>) => {
      onChange({ ...query, ...patch });
    },
    [onChange, query]
  );

  const updateAndRun = useCallback(
    (patch: Partial<DataBridgeQuery>) => {
      onChange({ ...query, ...patch });
      onRunQuery();
    },
    [onChange, onRunQuery, query]
  );

  // Load connections on mount
  useEffect(() => {
    let cancelled = false;
    datasource.getConnections().then((data) => {
      if (!cancelled) {
        setConnections(data);
      }
    }).catch(console.error);
    return () => { cancelled = true; };
  }, [datasource]);

  // Load databases when connection changes
  useEffect(() => {
    if (!query.connectionId) {
      return;
    }
    let cancelled = false;
    datasource.getDatabases(query.connectionId).then((data) => {
      if (!cancelled) {
        setDatabases(data);
      }
    }).catch(console.error);
    return () => { cancelled = true; };
  }, [datasource, query.connectionId]);

  // Load datasets when database changes
  useEffect(() => {
    if (!query.connectionId || !query.databaseName) {
      return;
    }
    let cancelled = false;
    datasource.getDatasets(query.connectionId, query.databaseName).then((data) => {
      if (!cancelled) {
        setDatasets(data);
      }
    }).catch(console.error);
    return () => { cancelled = true; };
  }, [datasource, query.connectionId, query.databaseName]);

  // Derive filtered options based on current query state (instead of clearing state in effects)
  const filteredDatabaseOptions = useMemo(() => {
    if (!query.connectionId) {
      return [];
    }
    return databases.map((d) => ({ label: d.name, value: d.name }));
  }, [query.connectionId, databases]);

  const filteredDatasetOptions = useMemo(() => {
    if (!query.connectionId || !query.databaseName) {
      return [];
    }
    return datasets.map((d) => ({ label: d.name, value: d.name }));
  }, [query.connectionId, query.databaseName, datasets]);

  const connectionOptions = connections.map((c) => ({
    label: c.name,
    value: c.id,
  }));

  return (
    <>
      {/* Mode & Strategy row */}
      <InlineFieldRow>
        <InlineField label="Mode" labelWidth={LABEL_WIDTH}>
          <RadioButtonGroup
            options={MODE_OPTIONS}
            value={query.mode ?? 'dataCatalog'}
            onChange={(value) => updateQuery({ mode: value })}
          />
        </InlineField>
        <InlineField label="Strategy" labelWidth={LABEL_WIDTH}>
          <RadioButtonGroup
            options={STRATEGY_OPTIONS}
            value={query.strategy ?? 'timeseries'}
            onChange={(value) => updateAndRun({ strategy: value })}
          />
        </InlineField>
        <InlineField label="Optimize Display" labelWidth={LABEL_WIDTH}>
          <Switch
            value={query.optimizeDisplay ?? true}
            onChange={(event) => updateAndRun({ optimizeDisplay: event.currentTarget.checked })}
          />
        </InlineField>
      </InlineFieldRow>

      {/* Raw mode: connection/database/dataset selectors */}
      {query.mode === 'raw' && (
        <InlineFieldRow>
          <InlineField label="Connection" labelWidth={LABEL_WIDTH}>
            <Combobox
              options={connectionOptions}
              value={query.connectionId ?? null}
              onChange={(option) =>
                updateQuery({
                  connectionId: option?.value,
                  databaseName: undefined,
                  datasetName: undefined,
                })
              }
              placeholder="Select connection..."
              isClearable
              width={24}
            />
          </InlineField>
          <InlineField label="Database" labelWidth={LABEL_WIDTH}>
            <Combobox
              options={filteredDatabaseOptions}
              value={query.databaseName ?? null}
              onChange={(option) =>
                updateQuery({
                  databaseName: option?.value,
                  datasetName: undefined,
                })
              }
              placeholder="Select database..."
              isClearable
              disabled={!query.connectionId}
              width={24}
            />
          </InlineField>
          <InlineField label="Dataset" labelWidth={LABEL_WIDTH}>
            <Combobox
              options={filteredDatasetOptions}
              value={query.datasetName ?? null}
              onChange={(option) => updateAndRun({ datasetName: option?.value })}
              placeholder="Select dataset..."
              isClearable
              disabled={!query.databaseName}
              width={24}
            />
          </InlineField>
        </InlineFieldRow>
      )}

      {/* DataCatalog mode placeholder */}
      {query.mode === 'dataCatalog' && (
        <InlineFieldRow>
          <InlineField label="Catalog Entries" labelWidth={LABEL_WIDTH} tooltip="Select tags from the DataCatalog">
            <div style={{ padding: '6px 0', color: '#888' }}>Asset tree browser (Phase 2)</div>
          </InlineField>
        </InlineFieldRow>
      )}

      {/* Aggregation */}
      <InlineFieldRow>
        <InlineField label="Aggregation" labelWidth={LABEL_WIDTH}>
          <Combobox
            options={AGGREGATION_OPTIONS}
            value={query.aggregation ?? 'avg'}
            onChange={(option) => updateAndRun({ aggregation: option.value as AggregationFunction })}
            width={20}
          />
        </InlineField>
      </InlineFieldRow>
    </>
  );
}

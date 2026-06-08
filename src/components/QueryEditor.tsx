import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { css } from '@emotion/css';
import { Combobox, Collapse, InlineField, InlineFieldRow, RadioButtonGroup, useStyles2 } from '@grafana/ui';
import { GrafanaTheme2, QueryEditorProps, SelectableValue } from '@grafana/data';

import { DataSource } from '../datasource';
import {
  CatalogEntry,
  ColumnInfo,
  DataBridgeOptions,
  DataBridgeQuery,
  DatabaseInfo,
  DatasetInfo,
  DisplayNamePreset,
  QueryMode,
  QueryStrategy,
  SelectDefinition,
  SourceConnection,
  TagOperation,
  timeWindowToSeconds,
} from '../types';
import { useAssetTree } from '../hooks/useAssetTree';
import { useRowEstimate } from '../hooks/useRowEstimate';
import { AssetTree } from './AssetTree';
import { SelectedTags } from './SelectedTags';
import { RawColumnSelector } from './RawColumnSelector';
import { DisplayNamePicker } from './DisplayNamePicker';
import { SafetyBanner } from './SafetyBanner';
import { QueryOptions } from './QueryOptions';
import { TransformsEditor } from './TransformsEditor';

type Props = QueryEditorProps<DataSource, DataBridgeQuery, DataBridgeOptions>;

const MODE_OPTIONS: Array<SelectableValue<QueryMode>> = [
  { label: 'DataCatalog', value: 'dataCatalog' },
  { label: 'Raw', value: 'raw' },
];

const STRATEGY_OPTIONS: Array<SelectableValue<QueryStrategy>> = [
  { label: 'Time Series', value: 'timeseries' },
  { label: 'Table', value: 'table' },
];

const LABEL_WIDTH = 16;

export function QueryEditor({ query, onChange, onRunQuery, datasource, range }: Props) {
  const styles = useStyles2(getStyles);

  // Raw mode state
  const [connections, setConnections] = useState<SourceConnection[]>([]);
  const [databases, setDatabases] = useState<DatabaseInfo[]>([]);
  const [datasets, setDatasets] = useState<DatasetInfo[]>([]);
  const [schemaColumns, setSchemaColumns] = useState<ColumnInfo[]>([]);

  // DataCatalog mode state
  const assetTree = useAssetTree(datasource);
  const [catalogEntries, setCatalogEntries] = useState<CatalogEntry[]>([]);

  // UI state
  const [isDisplayOpen, setIsDisplayOpen] = useState(false);

  // Settings
  const maxRawRows = datasource.settings.maxRawRows ?? 50_000;
  const hardLimitRows = datasource.settings.hardLimitRows ?? 1_000_000;

  const columnCount = (query.select ?? []).length || 1;

  // Derive optimizeDisplay from tags: true if any tag is not 'none'
  const optimizeDisplay = useMemo(() => {
    const select = query.select ?? [];
    if (select.length === 0) {
      return true;
    }
    return select.some((s) => (s.aggregation ?? 'optimized') !== 'none');
  }, [query.select]);

  // Detect multi-dataset: WHERE filters are disabled when tags span different datasets
  const isMultiDataset = useMemo(() => {
    if (catalogEntries.length <= 1) {
      return false;
    }
    const keys = new Set<string>();
    for (const entry of catalogEntries) {
      const connId = entry.sourceConnection?.id ?? entry.sourceConnectionId ?? '';
      const db = entry.sourceParams?.database ?? '';
      const ds = entry.sourceParams?.dataset ?? '';
      keys.add(`${connId}|${db}|${ds}`);
    }
    return keys.size > 1;
  }, [catalogEntries]);

  // Row estimation
  const rowEstimate = useRowEstimate({
    timeRange: range,
    columnCount,
    optimizeDisplay,
    maxDataPoints: 1000, // Grafana typical panel width
    maxRawRows,
    hardLimitRows,
    manualTimeWindowSeconds: timeWindowToSeconds(query.timeWindowInterval, query.timeWindowUnit),
  });

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

  // --- Raw mode data loading ---

  useEffect(() => {
    let cancelled = false;
    datasource.getConnections().then((data) => {
      if (!cancelled) { setConnections(data); }
    }).catch(() => {}); // Silently ignore — DataCatalog may be down
    return () => { cancelled = true; };
  }, [datasource]);

  useEffect(() => {
    if (!query.connectionId) { return; }
    let cancelled = false;
    datasource.getDatabases(query.connectionId).then((data) => {
      if (!cancelled) { setDatabases(data); }
    }).catch(console.error);
    return () => { cancelled = true; };
  }, [datasource, query.connectionId]);

  useEffect(() => {
    if (!query.connectionId || !query.databaseName) { return; }
    let cancelled = false;
    datasource.getDatasets(query.connectionId, query.databaseName).then((data) => {
      if (!cancelled) { setDatasets(data); }
    }).catch(console.error);
    return () => { cancelled = true; };
  }, [datasource, query.connectionId, query.databaseName]);

  // Load schema columns when dataset is selected in Raw mode
  useEffect(() => {
    if (!query.connectionId || !query.databaseName || !query.datasetName) {
      // eslint-disable-next-line react-hooks/set-state-in-effect -- reset state on dependency change
      setSchemaColumns([]);
      return;
    }
    let cancelled = false;
    datasource.getSchema(query.connectionId, query.databaseName, query.datasetName).then((schema) => {
      if (!cancelled) { setSchemaColumns(schema.columns ?? []); }
    }).catch(console.error);
    return () => { cancelled = true; };
  }, [datasource, query.connectionId, query.databaseName, query.datasetName]);

  // --- DataCatalog mode: load entries for selected IDs ---

  useEffect(() => {
    const ids = (query.select ?? [])
      .map((s) => s.catalogEntryId)
      .filter((id): id is string => !!id);
    if (ids.length === 0) {
      return;
    }
    let cancelled = false;
    datasource.getCatalogEntries({ ids: ids.join(',') }).then((data) => {
      if (!cancelled) { setCatalogEntries(data ?? []); }
    }).catch(() => {}); // Silently ignore — DataCatalog may be down
    return () => { cancelled = true; };
  }, [datasource, query.select]);

  // --- Derived options ---

  const connectionOptions = useMemo(
    () => connections.map((c) => ({ label: c.name, value: c.id })),
    [connections]
  );

  const filteredDatabaseOptions = useMemo(() => {
    if (!query.connectionId) { return []; }
    return databases.map((d) => ({ label: d.name, value: d.name }));
  }, [query.connectionId, databases]);

  const filteredDatasetOptions = useMemo(() => {
    if (!query.connectionId || !query.databaseName) { return []; }
    return datasets.map((d) => ({ label: d.name, value: d.name }));
  }, [query.connectionId, query.databaseName, datasets]);

  // --- Handlers ---

  const selectedEntryIds = useMemo(() => {
    return new Set(
      (query.select ?? []).map((s) => s.catalogEntryId).filter((id): id is string => !!id)
    );
  }, [query.select]);

  const handleSelectEntry = useCallback(
    (entry: CatalogEntry) => {
      const currentSelect = query.select ?? [];
      const alreadySelected = currentSelect.some((s) => s.catalogEntryId === entry.id);

      if (alreadySelected) {
        const nextSelect = currentSelect.filter((s) => s.catalogEntryId !== entry.id);
        updateAndRun({ select: nextSelect });
      } else {
        const newItem: SelectDefinition = {
          catalogEntryId: entry.id,
          column: entry.sourceParams?.column ?? entry.name,
          dataType: entry.dataType,
          aggregation: 'optimized',
        };
        updateAndRun({ select: [...currentSelect, newItem], optimizeDisplay: true });
      }
    },
    [query.select, updateAndRun]
  );

  const handleRemoveTag = useCallback(
    (index: number) => {
      const nextSelect = [...(query.select ?? [])];
      nextSelect.splice(index, 1);
      const hasAggregation = nextSelect.some((s) => (s.aggregation ?? 'optimized') !== 'none');
      updateAndRun({ select: nextSelect, optimizeDisplay: hasAggregation });
    },
    [query.select, updateAndRun]
  );

  const handleAggregationChange = useCallback(
    (index: number, operation: TagOperation) => {
      const nextSelect = [...(query.select ?? [])];
      nextSelect[index] = { ...nextSelect[index], aggregation: operation };
      // Derive optimizeDisplay for the backend
      const hasAggregation = nextSelect.some((s) => (s.aggregation ?? 'optimized') !== 'none');
      updateAndRun({ select: nextSelect, optimizeDisplay: hasAggregation });
    },
    [query.select, updateAndRun]
  );

  // --- Raw mode column handlers ---

  const handleToggleRawColumn = useCallback(
    (column: ColumnInfo) => {
      const currentSelect = query.select ?? [];
      const existingIndex = currentSelect.findIndex((s) => s.column === column.name);

      if (existingIndex >= 0) {
        const nextSelect = currentSelect.filter((_, i) => i !== existingIndex);
        const hasAggregation = nextSelect.some((s) => (s.aggregation ?? 'optimized') !== 'none');
        updateAndRun({ select: nextSelect, optimizeDisplay: hasAggregation || nextSelect.length === 0 });
      } else {
        const newItem: SelectDefinition = {
          column: column.name,
          dataType: column.type,
          aggregation: 'optimized',
        };
        updateAndRun({ select: [...currentSelect, newItem], optimizeDisplay: true });
      }
    },
    [query.select, updateAndRun]
  );

  const handleRemoveRawColumn = useCallback(
    (index: number) => {
      const nextSelect = [...(query.select ?? [])];
      nextSelect.splice(index, 1);
      const hasAggregation = nextSelect.some((s) => (s.aggregation ?? 'optimized') !== 'none');
      updateAndRun({ select: nextSelect, optimizeDisplay: hasAggregation || nextSelect.length === 0 });
    },
    [query.select, updateAndRun]
  );

  const handleRawAggregationChange = useCallback(
    (index: number, operation: TagOperation) => {
      const nextSelect = [...(query.select ?? [])];
      nextSelect[index] = { ...nextSelect[index], aggregation: operation };
      const hasAggregation = nextSelect.some((s) => (s.aggregation ?? 'optimized') !== 'none');
      updateAndRun({ select: nextSelect, optimizeDisplay: hasAggregation });
    },
    [query.select, updateAndRun]
  );

  return (
    <div className={styles.container}>
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
      </InlineFieldRow>

      {/* Safety banner */}
      <SafetyBanner
        estimate={rowEstimate}
        optimizeDisplay={optimizeDisplay}
        maxRawRows={maxRawRows}
      />

      {/* DataCatalog mode: asset tree + selected tags */}
      {(query.mode ?? 'dataCatalog') === 'dataCatalog' && (
        <div className={styles.catalogSection}>
          <div className={styles.twoColumn}>
            <div className={styles.treePanel}>
              <AssetTree
                trees={assetTree.trees}
                flatNodes={assetTree.flatNodes}
                labels={assetTree.labels}
                filteredEntries={assetTree.filteredEntries}
                loading={assetTree.loading}
                error={assetTree.error}
                searchQuery={assetTree.searchQuery}
                labelFilter={assetTree.labelFilter}
                selectedEntryIds={selectedEntryIds}
                selectedTreeId={assetTree.selectedTreeId}
                onSearchChange={assetTree.setSearchQuery}
                onLabelFilterChange={assetTree.setLabelFilter}
                onTreeChange={assetTree.setSelectedTreeId}
                onToggleNode={assetTree.toggleNode}
                onExpandAll={assetTree.expandAll}
                onCollapseAll={assetTree.collapseAll}
                onRefresh={assetTree.refresh}
                onSelectEntry={handleSelectEntry}
              />
            </div>
            <div className={styles.selectedPanel}>
              <SelectedTags
                select={query.select ?? []}
                entries={catalogEntries}
                displayNamePreset={query.displayNamePreset ?? 'entryName'}
                displayNamePattern={query.displayNamePattern ?? ''}
                assetPaths={assetTree.assetPaths}
                onRemove={handleRemoveTag}
                onAggregationChange={handleAggregationChange}
              />
            </div>
          </div>
        </div>
      )}

      {/* Raw mode: connection/database/dataset selectors + column picker */}
      {query.mode === 'raw' && (
        <>
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
                    select: [],
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
                    select: [],
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
                onChange={(option) => updateAndRun({ datasetName: option?.value, select: [] })}
                placeholder="Select dataset..."
                isClearable
                disabled={!query.databaseName}
                width={24}
              />
            </InlineField>
          </InlineFieldRow>

          {schemaColumns.length > 0 && (
            <RawColumnSelector
              columns={schemaColumns}
              select={query.select ?? []}
              onToggleColumn={handleToggleRawColumn}
              onRemove={handleRemoveRawColumn}
              onAggregationChange={handleRawAggregationChange}
            />
          )}
        </>
      )}

      {/* Query options: aggregation, filters, order, advanced */}
      <QueryOptions
        query={query}
        onUpdate={updateQuery}
        onUpdateAndRun={updateAndRun}
        isMultiDataset={isMultiDataset}
        optimizeDisplay={optimizeDisplay}
      />

      <TransformsEditor query={query} onUpdateAndRun={updateAndRun} />

      {/* Display name configuration (collapsible) */}
      <Collapse
        label={displaySummary(query.displayNamePreset ?? 'entryName')}
        isOpen={isDisplayOpen}
        onToggle={() => setIsDisplayOpen(!isDisplayOpen)}
      >
        <DisplayNamePicker
          preset={query.displayNamePreset ?? 'entryName'}
          pattern={query.displayNamePattern ?? ''}
          entries={catalogEntries}
          onPresetChange={(preset) => updateAndRun({ displayNamePreset: preset })}
          onPatternChange={(pattern) => updateAndRun({ displayNamePattern: pattern })}
        />
      </Collapse>
    </div>
  );
}


function displaySummary(preset: DisplayNamePreset): string {
  switch (preset) {
    case 'tagLevel1': return 'Display: Tag Level 1';
    case 'descriptionEn': return 'Display: Description (EN)';
    case 'descriptionDe': return 'Display: Description (DE)';
    case 'assetPath': return 'Display: Asset Path';
    case 'custom': return 'Display: Custom Pattern';
    default: return 'Display: Entry Name';
  }
}

function getStyles(theme: GrafanaTheme2) {
  return {
    container: css({
      display: 'flex',
      flexDirection: 'column',
      gap: theme.spacing(0.5),
    }),
    catalogSection: css({
      marginBottom: theme.spacing(0.5),
    }),
    twoColumn: css({
      display: 'flex',
      gap: theme.spacing(1),
      minHeight: '200px',
    }),
    treePanel: css({
      flex: 3,
      minWidth: 0,
    }),
    selectedPanel: css({
      flex: 2,
      minWidth: 0,
    }),
  };
}

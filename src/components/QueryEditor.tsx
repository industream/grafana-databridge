import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { css } from '@emotion/css';
import { Combobox, Collapse, InlineField, InlineFieldRow, RadioButtonGroup, useStyles2 } from '@grafana/ui';
import { GrafanaTheme2, QueryEditorProps, SelectableValue } from '@grafana/data';

import { DataSource } from '../datasource';
import {
  CatalogEntry,
  DataBridgeOptions,
  DataBridgeQuery,
  DatabaseInfo,
  DatasetInfo,
  DisplayNamePreset,
  QueryMode,
  QueryStrategy,
  SelectDefinition,
  SourceConnection,
  AggregationFunction,
  AggregationOrNone,
} from '../types';
import { useAssetTree } from '../hooks/useAssetTree';
import { useRowEstimate } from '../hooks/useRowEstimate';
import { AssetTree } from './AssetTree';
import { SelectedTags } from './SelectedTags';
import { DisplayNamePicker } from './DisplayNamePicker';
import { SafetyBanner } from './SafetyBanner';
import { QueryOptions } from './QueryOptions';

type Props = QueryEditorProps<DataSource, DataBridgeQuery, DataBridgeOptions>;

const MODE_OPTIONS: Array<SelectableValue<QueryMode>> = [
  { label: 'DataCatalog', value: 'dataCatalog' },
  { label: 'Raw', value: 'raw' },
];

const STRATEGY_OPTIONS: Array<SelectableValue<QueryStrategy>> = [
  { label: 'Time Series', value: 'timeseries' },
  { label: 'Table', value: 'table' },
];

type AggregationOption = AggregationOrNone | 'auto';

const AGGREGATION_OPTIONS: Array<{ label: string; value: AggregationOption }> = [
  { label: 'None (raw data)', value: 'none' },
  { label: 'Optimized Display', value: 'auto' },
  { label: 'Average', value: 'avg' },
  { label: 'Minimum', value: 'min' },
  { label: 'Maximum', value: 'max' },
  { label: 'Sum', value: 'sum' },
  { label: 'Count', value: 'count' },
  { label: 'First', value: 'first' },
  { label: 'Last', value: 'last' },
];

const LABEL_WIDTH = 16;

export function QueryEditor({ query, onChange, onRunQuery, datasource, range }: Props) {
  const styles = useStyles2(getStyles);

  // Raw mode state
  const [connections, setConnections] = useState<SourceConnection[]>([]);
  const [databases, setDatabases] = useState<DatabaseInfo[]>([]);
  const [datasets, setDatasets] = useState<DatasetInfo[]>([]);

  // DataCatalog mode state
  const assetTree = useAssetTree(datasource);
  const [catalogEntries, setCatalogEntries] = useState<CatalogEntry[]>([]);

  // UI state
  const [isDisplayOpen, setIsDisplayOpen] = useState(false);

  // Settings
  const maxRawRows = datasource.settings.maxRawRows ?? 50_000;
  const hardLimitRows = datasource.settings.hardLimitRows ?? 1_000_000;

  const columnCount = (query.select ?? []).length || 1;

  // Row estimation
  const rowEstimate = useRowEstimate({
    timeRange: range,
    columnCount,
    optimizeDisplay: query.optimizeDisplay ?? true,
    maxDataPoints: 1000, // Grafana typical panel width
    maxRawRows,
    hardLimitRows,
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

  // Derive aggregation display value for the dropdown
  const defaultAgg = datasource.settings.defaultAggregation ?? 'avg';
  const currentAggregation: AggregationOption = !(query.optimizeDisplay ?? true)
    ? 'none'
    : query.aggregation
      ? query.aggregation
      : 'auto';

  // --- Raw mode data loading ---

  useEffect(() => {
    let cancelled = false;
    datasource.getConnections().then((data) => {
      if (!cancelled) { setConnections(data); }
    }).catch(console.error);
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
    }).catch(console.error);
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
        const defaultAgg = defaultAggregationForLabel(entry.labels);
        const newItem: SelectDefinition = {
          catalogEntryId: entry.id,
          column: entry.sourceParams?.column ?? entry.name,
          aggregation: defaultAgg,
        };
        updateAndRun({ select: [...currentSelect, newItem] });
      }
    },
    [query.select, updateAndRun]
  );

  const handleRemoveTag = useCallback(
    (index: number) => {
      const nextSelect = [...(query.select ?? [])];
      nextSelect.splice(index, 1);
      updateAndRun({ select: nextSelect });
    },
    [query.select, updateAndRun]
  );

  const handleAggregationChange = useCallback(
    (index: number, aggregation: AggregationFunction) => {
      const nextSelect = [...(query.select ?? [])];
      nextSelect[index] = { ...nextSelect[index], aggregation };
      updateAndRun({ select: nextSelect });
    },
    [query.select, updateAndRun]
  );

  return (
    <div className={styles.container}>
      {/* Mode, Strategy & Aggregation row */}
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
        <InlineField label="Aggregation" labelWidth={LABEL_WIDTH} tooltip="None = raw data, Optimized Display = default from config, or pick a specific function">
          <Combobox
            options={AGGREGATION_OPTIONS}
            value={currentAggregation}
            onChange={(option) => {
              const val = option.value as AggregationOption;
              if (val === 'none') {
                updateAndRun({ optimizeDisplay: false, aggregation: undefined });
              } else if (val === 'auto') {
                updateAndRun({ optimizeDisplay: true, aggregation: undefined });
              } else {
                updateAndRun({ optimizeDisplay: true, aggregation: val as AggregationFunction });
              }
            }}
            width={24}
          />
        </InlineField>
        {currentAggregation !== 'none' && rowEstimate?.timeWindowLabel && (
          <InlineField label="Window" labelWidth={LABEL_WIDTH}>
            <span className={styles.windowLabel}>{rowEstimate.timeWindowLabel} (auto)</span>
          </InlineField>
        )}
      </InlineFieldRow>

      {/* DataCatalog mode: asset tree + selected tags */}
      {(query.mode ?? 'dataCatalog') === 'dataCatalog' && (
        <div className={styles.catalogSection}>
          <div className={styles.twoColumn}>
            <div className={styles.treePanel}>
              <AssetTree
                flatNodes={assetTree.flatNodes}
                labels={assetTree.labels}
                filteredEntries={assetTree.filteredEntries}
                loading={assetTree.loading}
                error={assetTree.error}
                searchQuery={assetTree.searchQuery}
                labelFilter={assetTree.labelFilter}
                selectedEntryIds={selectedEntryIds}
                onSearchChange={assetTree.setSearchQuery}
                onLabelFilterChange={assetTree.setLabelFilter}
                onToggleNode={assetTree.toggleNode}
                onExpandAll={assetTree.expandAll}
                onCollapseAll={assetTree.collapseAll}
                onSelectEntry={handleSelectEntry}
              />
            </div>
            <div className={styles.selectedPanel}>
              <SelectedTags
                select={query.select ?? []}
                entries={catalogEntries}
                displayNamePreset={query.displayNamePreset ?? 'entryName'}
                displayNamePattern={query.displayNamePattern ?? ''}
                onRemove={handleRemoveTag}
                onAggregationChange={handleAggregationChange}
              />
            </div>
          </div>
        </div>
      )}

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

      {/* Safety banner */}
      <SafetyBanner
        estimate={rowEstimate}
        optimizeDisplay={query.optimizeDisplay ?? true}
        maxRawRows={maxRawRows}
      />

      {/* Query options: aggregation, filters, order, advanced */}
      <QueryOptions
        query={query}
        onUpdate={updateQuery}
        onUpdateAndRun={updateAndRun}
      />

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
          onPresetChange={(preset) => updateQuery({ displayNamePreset: preset })}
          onPatternChange={(pattern) => updateQuery({ displayNamePattern: pattern })}
        />
      </Collapse>
    </div>
  );
}

function defaultAggregationForLabel(labels: Array<{ name: string }>): AggregationFunction {
  if (labels.length === 0) {
    return 'avg';
  }
  switch (labels[0].name.toLowerCase()) {
    case 'analog': return 'avg';
    case 'digital': return 'last';
    case 'counter': return 'max';
    default: return 'avg';
  }
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
      flex: 1,
      minWidth: 0,
    }),
    selectedPanel: css({
      flex: 1,
      minWidth: 0,
    }),
    windowLabel: css({
      fontSize: theme.typography.bodySmall.fontSize,
      color: theme.colors.text.secondary,
      padding: '6px 0',
      display: 'inline-block',
    }),
  };
}

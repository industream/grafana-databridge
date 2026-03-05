import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { css } from '@emotion/css';
import { Combobox, Collapse, InlineField, InlineFieldRow, RadioButtonGroup, Switch, useStyles2 } from '@grafana/ui';
import { GrafanaTheme2, QueryEditorProps, SelectableValue } from '@grafana/data';

import { DataSource } from '../datasource';
import {
  AggregationFunction,
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
} from '../types';
import { useAssetTree } from '../hooks/useAssetTree';
import { AssetTree } from './AssetTree';
import { SelectedTags } from './SelectedTags';
import { DisplayNamePicker } from './DisplayNamePicker';

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
        // Deselect
        const nextSelect = currentSelect.filter((s) => s.catalogEntryId !== entry.id);
        updateAndRun({ select: nextSelect });
      } else {
        // Select with default aggregation based on label
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

  const handleDisplayPresetChange = useCallback(
    (preset: DisplayNamePreset) => {
      updateQuery({ displayNamePreset: preset });
    },
    [updateQuery]
  );

  const handleDisplayPatternChange = useCallback(
    (pattern: string) => {
      updateQuery({ displayNamePattern: pattern });
    },
    [updateQuery]
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
        <InlineField label="Optimize Display" labelWidth={LABEL_WIDTH}>
          <Switch
            value={query.optimizeDisplay ?? true}
            onChange={(event) => updateAndRun({ optimizeDisplay: event.currentTarget.checked })}
          />
        </InlineField>
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
          onPresetChange={handleDisplayPresetChange}
          onPatternChange={handleDisplayPatternChange}
        />
      </Collapse>
    </div>
  );
}

function defaultAggregationForLabel(labels: string[]): AggregationFunction {
  if (labels.length === 0) {
    return 'avg';
  }
  switch (labels[0].toLowerCase()) {
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
  };
}

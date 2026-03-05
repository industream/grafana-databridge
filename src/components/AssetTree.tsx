import React from 'react';
import { css } from '@emotion/css';
import { Badge, Combobox, FilterInput, Icon, IconButton, Spinner, Stack, useStyles2 } from '@grafana/ui';
import { GrafanaTheme2 } from '@grafana/data';

import { CatalogEntry, Label } from '../types';
import { FlatTreeNode } from '../hooks/useAssetTree';

interface AssetTreeProps {
  flatNodes: FlatTreeNode[];
  labels: Label[];
  filteredEntries: CatalogEntry[];
  loading: boolean;
  error: string | null;
  searchQuery: string;
  labelFilter: string | null;
  selectedEntryIds: Set<string>;
  onSearchChange: (query: string) => void;
  onLabelFilterChange: (label: string | null) => void;
  onToggleNode: (nodeId: string) => void;
  onExpandAll: () => void;
  onCollapseAll: () => void;
  onSelectEntry: (entry: CatalogEntry) => void;
}

export function AssetTree({
  flatNodes,
  labels,
  filteredEntries,
  loading,
  error,
  searchQuery,
  labelFilter,
  selectedEntryIds,
  onSearchChange,
  onLabelFilterChange,
  onToggleNode,
  onExpandAll,
  onCollapseAll,
  onSelectEntry,
}: AssetTreeProps) {
  const styles = useStyles2(getStyles);

  const labelOptions = labels.map((l) => ({ label: l.name, value: l.name }));

  if (loading) {
    return (
      <div className={styles.container}>
        <Stack direction="row" alignItems="center" gap={1}>
          <Spinner size="sm" />
          <span>Loading asset tree...</span>
        </Stack>
      </div>
    );
  }

  if (error) {
    return (
      <div className={styles.container}>
        <span className={styles.error}>{error}</span>
      </div>
    );
  }

  const isSearchMode = searchQuery.trim().length > 0;

  return (
    <div className={styles.container}>
      {/* Search bar + label filter */}
      <div className={styles.toolbar}>
        <FilterInput
          placeholder="Search tags..."
          value={searchQuery}
          onChange={onSearchChange}
          className={styles.searchInput}
        />
        <Combobox
          options={labelOptions}
          value={labelFilter}
          onChange={(option) => onLabelFilterChange(option?.value ?? null)}
          placeholder="All labels"
          isClearable
          width={16}
        />
        <IconButton name="angle-double-down" tooltip="Expand all" onClick={onExpandAll} size="sm" />
        <IconButton name="angle-double-up" tooltip="Collapse all" onClick={onCollapseAll} size="sm" />
      </div>

      {/* Tree or search results */}
      <div className={styles.treeBody}>
        {isSearchMode ? (
          <SearchResults
            entries={filteredEntries}
            selectedEntryIds={selectedEntryIds}
            onSelectEntry={onSelectEntry}
          />
        ) : (
          <TreeNodes
            flatNodes={flatNodes}
            selectedEntryIds={selectedEntryIds}
            onToggleNode={onToggleNode}
            onSelectEntry={onSelectEntry}
            filteredEntries={filteredEntries}
          />
        )}
      </div>
    </div>
  );
}

// --- Tree nodes rendering ---

interface TreeNodesProps {
  flatNodes: FlatTreeNode[];
  selectedEntryIds: Set<string>;
  onToggleNode: (nodeId: string) => void;
  onSelectEntry: (entry: CatalogEntry) => void;
  filteredEntries: CatalogEntry[];
}

function TreeNodes({ flatNodes, selectedEntryIds, onToggleNode, onSelectEntry, filteredEntries }: TreeNodesProps) {
  const styles = useStyles2(getStyles);

  if (flatNodes.length === 0) {
    return <div className={styles.emptyState}>No asset dictionaries configured</div>;
  }

  // Build a map of nodeId -> entries for leaf rendering
  // For now, entries are matched by being in filteredEntries
  // TODO: Phase 2 enhancement — load entries per node from backend

  return (
    <>
      {flatNodes.map((node) => (
        <div key={node.id} className={styles.treeNode} style={{ paddingLeft: `${node.depth * 20 + 4}px` }}>
          <button className={styles.nodeToggle} onClick={() => onToggleNode(node.id)} type="button">
            <Icon name={node.isExpanded ? 'angle-down' : 'angle-right'} size="sm" />
          </button>
          <Icon name="folder" size="sm" className={styles.nodeIcon} />
          <span className={styles.nodeName}>{node.name}</span>
          {node.entryCount > 0 && (
            <Badge text={String(node.entryCount)} color="blue" className={styles.countBadge} />
          )}
        </div>
      ))}
    </>
  );
}

// --- Search results rendering ---

interface SearchResultsProps {
  entries: CatalogEntry[];
  selectedEntryIds: Set<string>;
  onSelectEntry: (entry: CatalogEntry) => void;
}

function SearchResults({ entries, selectedEntryIds, onSelectEntry }: SearchResultsProps) {
  const styles = useStyles2(getStyles);

  if (entries.length === 0) {
    return <div className={styles.emptyState}>No matching entries</div>;
  }

  return (
    <>
      <div className={styles.resultCount}>{entries.length} entries found</div>
      {entries.map((entry) => (
        <EntryRow
          key={entry.id}
          entry={entry}
          isSelected={selectedEntryIds.has(entry.id)}
          onSelect={onSelectEntry}
        />
      ))}
    </>
  );
}

// --- Single entry row ---

interface EntryRowProps {
  entry: CatalogEntry;
  isSelected: boolean;
  onSelect: (entry: CatalogEntry) => void;
}

function EntryRow({ entry, isSelected, onSelect }: EntryRowProps) {
  const styles = useStyles2(getStyles);

  return (
    <button
      className={`${styles.entryRow} ${isSelected ? styles.entryRowSelected : ''}`}
      onClick={() => onSelect(entry)}
      type="button"
    >
      <Icon name={isSelected ? 'check-circle' : 'circle'} size="sm" className={styles.entryCheckIcon} />
      <span className={styles.entryName}>{entry.name}</span>
      {entry.metadata?.tagLevel1 && <span className={styles.entryTag}>{entry.metadata.tagLevel1}</span>}
      {entry.labels.length > 0 && (
        <Badge text={entry.labels[0]} color={labelColor(entry.labels[0])} className={styles.entryLabel} />
      )}
    </button>
  );
}

function labelColor(label: string): 'blue' | 'green' | 'orange' | 'red' | 'purple' {
  switch (label.toLowerCase()) {
    case 'analog': return 'blue';
    case 'digital': return 'green';
    case 'counter': return 'orange';
    case 'event': return 'purple';
    default: return 'blue';
  }
}

// --- Styles ---

function getStyles(theme: GrafanaTheme2) {
  return {
    container: css({
      border: `1px solid ${theme.colors.border.weak}`,
      borderRadius: theme.shape.radius.default,
      backgroundColor: theme.colors.background.primary,
      minHeight: '200px',
      maxHeight: '400px',
      display: 'flex',
      flexDirection: 'column',
      overflow: 'hidden',
    }),
    toolbar: css({
      display: 'flex',
      alignItems: 'center',
      gap: theme.spacing(0.5),
      padding: theme.spacing(0.5),
      borderBottom: `1px solid ${theme.colors.border.weak}`,
      flexShrink: 0,
    }),
    searchInput: css({
      flex: 1,
    }),
    treeBody: css({
      flex: 1,
      overflowY: 'auto',
      padding: theme.spacing(0.5),
    }),
    treeNode: css({
      display: 'flex',
      alignItems: 'center',
      gap: theme.spacing(0.5),
      padding: `${theme.spacing(0.25)} 0`,
      minHeight: '28px',
    }),
    nodeToggle: css({
      background: 'none',
      border: 'none',
      cursor: 'pointer',
      padding: '2px',
      display: 'flex',
      alignItems: 'center',
      color: theme.colors.text.secondary,
      '&:hover': {
        color: theme.colors.text.primary,
      },
    }),
    nodeIcon: css({
      color: theme.colors.text.secondary,
      flexShrink: 0,
    }),
    nodeName: css({
      fontSize: theme.typography.bodySmall.fontSize,
      color: theme.colors.text.primary,
      flex: 1,
      overflow: 'hidden',
      textOverflow: 'ellipsis',
      whiteSpace: 'nowrap',
    }),
    countBadge: css({
      flexShrink: 0,
    }),
    resultCount: css({
      fontSize: theme.typography.bodySmall.fontSize,
      color: theme.colors.text.secondary,
      padding: `${theme.spacing(0.25)} ${theme.spacing(0.5)}`,
    }),
    entryRow: css({
      display: 'flex',
      alignItems: 'center',
      gap: theme.spacing(0.5),
      padding: `${theme.spacing(0.25)} ${theme.spacing(0.5)}`,
      cursor: 'pointer',
      background: 'none',
      border: 'none',
      width: '100%',
      textAlign: 'left',
      borderRadius: theme.shape.radius.default,
      '&:hover': {
        backgroundColor: theme.colors.action.hover,
      },
    }),
    entryRowSelected: css({
      backgroundColor: theme.colors.action.selected,
    }),
    entryCheckIcon: css({
      flexShrink: 0,
    }),
    entryName: css({
      fontSize: theme.typography.bodySmall.fontSize,
      color: theme.colors.text.primary,
      flex: 1,
      overflow: 'hidden',
      textOverflow: 'ellipsis',
      whiteSpace: 'nowrap',
    }),
    entryTag: css({
      fontSize: theme.typography.bodySmall.fontSize,
      color: theme.colors.text.disabled,
      flexShrink: 0,
      maxWidth: '200px',
      overflow: 'hidden',
      textOverflow: 'ellipsis',
      whiteSpace: 'nowrap',
    }),
    entryLabel: css({
      flexShrink: 0,
    }),
    emptyState: css({
      padding: theme.spacing(2),
      textAlign: 'center',
      color: theme.colors.text.secondary,
      fontSize: theme.typography.bodySmall.fontSize,
    }),
    error: css({
      color: theme.colors.error.text,
      padding: theme.spacing(1),
    }),
  };
}

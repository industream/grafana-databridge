import React from 'react';
import { css } from '@emotion/css';
import { Badge, Combobox, IconButton, Tooltip, useStyles2 } from '@grafana/ui';
import { GrafanaTheme2 } from '@grafana/data';

import { AggregationFunction, CatalogEntry, DisplayNamePreset, SelectDefinition } from '../types';
import { resolveDisplayName } from '../hooks/useDisplayName';

interface SelectedTagsProps {
  select: SelectDefinition[];
  entries: CatalogEntry[];
  displayNamePreset: DisplayNamePreset;
  displayNamePattern: string;
  onRemove: (index: number) => void;
  onAggregationChange: (index: number, aggregation: AggregationFunction) => void;
}

const AGGREGATION_OPTIONS = [
  { label: 'avg', value: 'avg' as const },
  { label: 'min', value: 'min' as const },
  { label: 'max', value: 'max' as const },
  { label: 'sum', value: 'sum' as const },
  { label: 'count', value: 'count' as const },
  { label: 'first', value: 'first' as const },
  { label: 'last', value: 'last' as const },
];

export function SelectedTags({
  select,
  entries,
  displayNamePreset,
  displayNamePattern,
  onRemove,
  onAggregationChange,
}: SelectedTagsProps) {
  const styles = useStyles2(getStyles);

  const entryMap = new Map(entries.map((e) => [e.id, e]));

  if (select.length === 0) {
    return <div className={styles.emptyState}>No tags selected. Use the tree or search to add tags.</div>;
  }

  return (
    <div className={styles.container}>
      {select.map((item, index) => {
        const entry = item.catalogEntryId ? entryMap.get(item.catalogEntryId) : undefined;
        const displayName = resolveDisplayName(entry, displayNamePreset, displayNamePattern, {
          column: item.column,
          aggregation: item.aggregation,
        });

        return (
          <div key={item.catalogEntryId ?? item.column ?? index} className={styles.tagRow}>
            <Tooltip content={entry?.metadata?.tagLevel1 ?? item.column ?? ''} placement="top">
              <span className={styles.tagName}>{displayName}</span>
            </Tooltip>

            {entry?.metadata?.unit && <span className={styles.unit}>{entry.metadata.unit}</span>}

            {entry && entry.labels.length > 0 && (
              <Badge text={entry.labels[0].name} color={labelColor(entry.labels[0].name)} className={styles.label} />
            )}

            <Combobox
              options={AGGREGATION_OPTIONS}
              value={item.aggregation ?? 'avg'}
              onChange={(option) => onAggregationChange(index, option.value as AggregationFunction)}
              width={10}
            />

            <IconButton name="times" tooltip="Remove" onClick={() => onRemove(index)} size="sm" />
          </div>
        );
      })}
    </div>
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

function getStyles(theme: GrafanaTheme2) {
  return {
    container: css({
      display: 'flex',
      flexDirection: 'column',
      gap: theme.spacing(0.25),
    }),
    tagRow: css({
      display: 'flex',
      alignItems: 'center',
      gap: theme.spacing(0.5),
      padding: `${theme.spacing(0.25)} ${theme.spacing(0.5)}`,
      borderRadius: theme.shape.radius.default,
      backgroundColor: theme.colors.background.secondary,
      minHeight: '32px',
    }),
    tagName: css({
      flex: 1,
      fontSize: theme.typography.bodySmall.fontSize,
      color: theme.colors.text.primary,
      overflow: 'hidden',
      textOverflow: 'ellipsis',
      whiteSpace: 'nowrap',
      cursor: 'default',
    }),
    unit: css({
      fontSize: theme.typography.bodySmall.fontSize,
      color: theme.colors.text.secondary,
      flexShrink: 0,
    }),
    label: css({
      flexShrink: 0,
    }),
    emptyState: css({
      padding: theme.spacing(1),
      color: theme.colors.text.secondary,
      fontSize: theme.typography.bodySmall.fontSize,
      fontStyle: 'italic',
    }),
  };
}

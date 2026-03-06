import React from 'react';
import { css } from '@emotion/css';
import { Badge, Combobox, IconButton, Tooltip, useStyles2 } from '@grafana/ui';
import { GrafanaTheme2 } from '@grafana/data';

import { CatalogEntry, DisplayNamePreset, SelectDefinition, TagOperation } from '../types';
import { resolveDisplayName } from '../hooks/useDisplayName';
import { dataTypeColor, labelColor } from '../utils/colors';

interface SelectedTagsProps {
  select: SelectDefinition[];
  entries: CatalogEntry[];
  displayNamePreset: DisplayNamePreset;
  displayNamePattern: string;
  assetPaths?: Record<string, string>;
  onRemove: (index: number) => void;
  onAggregationChange: (index: number, operation: TagOperation) => void;
}

const OPERATION_OPTIONS = [
  { label: 'Optimized', value: 'optimized' },
  { label: 'Raw', value: 'none' },
  { label: 'avg', value: 'avg' },
  { label: 'min', value: 'min' },
  { label: 'max', value: 'max' },
  { label: 'sum', value: 'sum' },
  { label: 'count', value: 'count' },
  { label: 'first', value: 'first' },
  { label: 'last', value: 'last' },
];

export function SelectedTags({
  select,
  entries,
  displayNamePreset,
  displayNamePattern,
  assetPaths,
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
        const rawPath = item.catalogEntryId ? assetPaths?.[item.catalogEntryId] : undefined;
        const entryPath = rawPath ? `${rawPath} > ${entry?.name ?? ''}` : undefined;
        const displayName = resolveDisplayName(entry, displayNamePreset, displayNamePattern, {
          column: item.column,
          aggregation: item.aggregation,
          assetPath: entryPath,
        });

        const meta = entry?.metadata;
        const tooltipLines = [
          meta?.tagLevel1 ?? item.column ?? '',
          meta?.description?.['en-US'] ? `${meta.description['en-US']}` : '',
          meta?.unit ? `Unit: ${meta.unit}` : '',
          meta?.min != null && meta?.max != null ? `Range: ${meta.min} – ${meta.max}` : '',
          meta?.decimals != null ? `Decimals: ${meta.decimals}` : '',
        ].filter(Boolean).join('\n');

        const rangeLabel = meta?.min != null && meta?.max != null
          ? `${meta.min}..${meta.max}`
          : undefined;

        return (
          <div key={item.catalogEntryId ?? item.column ?? index} className={styles.tagRow}>
            <Tooltip content={tooltipLines} placement="top">
              <span className={styles.tagName}>{displayName}</span>
            </Tooltip>

            {entry?.dataType && (
              <Badge text={entry.dataType} color={dataTypeColor(entry.dataType)} className={styles.typeBadge} />
            )}

            {meta?.unit && <span className={styles.unit}>{meta.unit}</span>}

            {rangeLabel && <span className={styles.range}>[{rangeLabel}]</span>}

            {entry && entry.labels.length > 0 && (
              <Badge text={entry.labels[0].name} color={labelColor(entry.labels[0].name)} className={styles.label} />
            )}
            {entry && entry.labels.length > 1 && (
              <Tooltip content={entry.labels.map((l) => l.name).join(', ')} placement="top">
                <Badge text={`+${entry.labels.length - 1}`} color="blue" className={styles.label} />
              </Tooltip>
            )}

            <Combobox
              options={OPERATION_OPTIONS}
              value={item.aggregation ?? 'optimized'}
              onChange={(option) => onAggregationChange(index, option.value as TagOperation)}
              width={12}
            />

            <IconButton name="times" tooltip="Remove" onClick={() => onRemove(index)} size="sm" />
          </div>
        );
      })}
    </div>
  );
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
    range: css({
      fontSize: theme.typography.bodySmall.fontSize,
      color: theme.colors.text.disabled,
      flexShrink: 0,
    }),
    typeBadge: css({
      flexShrink: 0,
      transform: 'scale(0.85)',
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

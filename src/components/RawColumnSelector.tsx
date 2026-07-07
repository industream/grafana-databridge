import React from 'react';
import { css } from '@emotion/css';
import { Badge, Combobox, IconButton, useStyles2 } from '@grafana/ui';
import { GrafanaTheme2 } from '@grafana/data';

import { ColumnInfo, SelectDefinition, TagOperation } from '../types';
import { dataTypeColor } from '../utils/colors';

interface RawColumnSelectorProps {
  columns: ColumnInfo[];
  select: SelectDefinition[];
  onToggleColumn: (column: ColumnInfo) => void;
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
  { label: 'stddev', value: 'stddev' },
  { label: 'stddev (pop)', value: 'stddev_pop' },
  { label: 'variance', value: 'variance' },
  { label: 'variance (pop)', value: 'var_pop' },
];

export function RawColumnSelector({
  columns,
  select,
  onToggleColumn,
  onRemove,
  onAggregationChange,
}: RawColumnSelectorProps) {
  const styles = useStyles2(getStyles);

  const selectedColumns = new Set(select.map((s) => s.column));

  // Filter out time column — it's always included automatically
  const selectableColumns = columns.filter((c) => c.name !== 'time');

  return (
    <div className={styles.container}>
      {/* Available columns */}
      <div className={styles.availableColumns}>
        {selectableColumns.map((col) => {
          const isSelected = selectedColumns.has(col.name);
          return (
            <button
              key={col.name}
              type="button"
              className={isSelected ? styles.columnChipSelected : styles.columnChip}
              onClick={() => onToggleColumn(col)}
            >
              <span>{col.name}</span>
              <Badge text={col.type} color={dataTypeColor(col.type)} className={styles.typeBadge} />
            </button>
          );
        })}
      </div>

      {/* Selected columns with aggregation */}
      {select.length > 0 && (
        <div className={styles.selectedList}>
          {select.map((item, index) => (
            <div key={item.column ?? index} className={styles.tagRow}>
              <span className={styles.tagName}>{item.column}</span>
              <span className={styles.typeLabel}>{item.dataType}</span>

              <Combobox
                options={OPERATION_OPTIONS}
                value={item.aggregation ?? 'optimized'}
                onChange={(option) => onAggregationChange(index, option.value as TagOperation)}
                width={12}
              />

              <IconButton name="times" tooltip="Remove" onClick={() => onRemove(index)} size="sm" />
            </div>
          ))}
        </div>
      )}

      {select.length === 0 && (
        <div className={styles.emptyState}>Click columns above to add them to the query.</div>
      )}
    </div>
  );
}

function getStyles(theme: GrafanaTheme2) {
  return {
    container: css({
      display: 'flex',
      flexDirection: 'column',
      gap: theme.spacing(0.5),
    }),
    availableColumns: css({
      display: 'flex',
      flexWrap: 'wrap',
      gap: theme.spacing(0.5),
    }),
    columnChip: css({
      display: 'inline-flex',
      alignItems: 'center',
      gap: theme.spacing(0.5),
      padding: `${theme.spacing(0.25)} ${theme.spacing(0.75)}`,
      borderRadius: theme.shape.radius.pill,
      border: `1px solid ${theme.colors.border.weak}`,
      background: theme.colors.background.primary,
      cursor: 'pointer',
      fontSize: theme.typography.bodySmall.fontSize,
      color: theme.colors.text.secondary,
      transition: 'all 0.15s ease',
      '&:hover': {
        borderColor: theme.colors.primary.border,
        color: theme.colors.text.primary,
      },
    }),
    columnChipSelected: css({
      display: 'inline-flex',
      alignItems: 'center',
      gap: theme.spacing(0.5),
      padding: `${theme.spacing(0.25)} ${theme.spacing(0.75)}`,
      borderRadius: theme.shape.radius.pill,
      border: `1px solid ${theme.colors.primary.border}`,
      background: theme.colors.primary.transparent,
      cursor: 'pointer',
      fontSize: theme.typography.bodySmall.fontSize,
      color: theme.colors.primary.text,
      fontWeight: theme.typography.fontWeightMedium,
    }),
    typeBadge: css({
      transform: 'scale(0.85)',
    }),
    selectedList: css({
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
    }),
    typeLabel: css({
      fontSize: theme.typography.bodySmall.fontSize,
      color: theme.colors.text.disabled,
      flexShrink: 0,
    }),
    emptyState: css({
      padding: theme.spacing(0.5),
      color: theme.colors.text.secondary,
      fontSize: theme.typography.bodySmall.fontSize,
      fontStyle: 'italic',
    }),
  };
}

import React, { ChangeEvent, useState } from 'react';
import { css } from '@emotion/css';
import { Collapse, Combobox, InlineField, InlineFieldRow, Input, useStyles2 } from '@grafana/ui';
import { GrafanaTheme2 } from '@grafana/data';

import { DataBridgeQuery, WhereCondition } from '../types';

interface QueryOptionsProps {
  query: DataBridgeQuery;
  onUpdate: (patch: Partial<DataBridgeQuery>) => void;
  onUpdateAndRun: (patch: Partial<DataBridgeQuery>) => void;
}

const ORDER_DIRECTION_OPTIONS = [
  { label: 'ASC', value: 'asc' as const },
  { label: 'DESC', value: 'desc' as const },
];

const LABEL_WIDTH = 14;

export function QueryOptions({ query, onUpdate, onUpdateAndRun }: QueryOptionsProps) {
  const styles = useStyles2(getStyles);

  const [isFiltersOpen, setIsFiltersOpen] = useState(false);
  const [isOrderOpen, setIsOrderOpen] = useState(false);
  const [isAdvancedOpen, setIsAdvancedOpen] = useState(false);

  const filterSummary = (query.where?.length ?? 0) > 0
    ? `${query.where!.length} filter(s)`
    : '(none)';

  const orderSummary = query.orderByColumn
    ? `${query.orderByColumn} ${query.orderByDirection ?? 'asc'}`
    : 'time asc (default)';

  const advancedSummary = [
    query.limit ? `limit ${query.limit}` : null,
    query.offset ? `offset ${query.offset}` : null,
  ]
    .filter(Boolean)
    .join(', ') || '(none)';

  return (
    <div className={styles.container}>
      {/* Filters (WHERE) — collapsible */}
      <Collapse
        label={`Filters (WHERE): ${filterSummary}`}
        isOpen={isFiltersOpen}
        onToggle={() => setIsFiltersOpen(!isFiltersOpen)}
      >
        <FilterEditor
          conditions={query.where ?? []}
          onChange={(where) => onUpdate({ where })}
          onRunQuery={() => onUpdateAndRun({})}
        />
      </Collapse>

      {/* Order By — collapsible */}
      <Collapse
        label={`Order By: ${orderSummary}`}
        isOpen={isOrderOpen}
        onToggle={() => setIsOrderOpen(!isOrderOpen)}
      >
        <InlineFieldRow>
          <InlineField label="Column" labelWidth={LABEL_WIDTH}>
            <Input
              value={query.orderByColumn ?? ''}
              onChange={(e: ChangeEvent<HTMLInputElement>) =>
                onUpdate({ orderByColumn: e.target.value || undefined })
              }
              placeholder="time"
              width={20}
            />
          </InlineField>
          <InlineField label="Direction" labelWidth={LABEL_WIDTH}>
            <Combobox
              options={ORDER_DIRECTION_OPTIONS}
              value={query.orderByDirection ?? 'asc'}
              onChange={(option) => onUpdateAndRun({ orderByDirection: option.value as 'asc' | 'desc' })}
              width={12}
            />
          </InlineField>
        </InlineFieldRow>
      </Collapse>

      {/* Advanced (limit/offset) — collapsible */}
      <Collapse
        label={`Advanced: ${advancedSummary}`}
        isOpen={isAdvancedOpen}
        onToggle={() => setIsAdvancedOpen(!isAdvancedOpen)}
      >
        <InlineFieldRow>
          <InlineField label="Limit" labelWidth={LABEL_WIDTH}>
            <Input
              type="number"
              value={query.limit ?? ''}
              onChange={(e: ChangeEvent<HTMLInputElement>) => {
                const v = parseInt(e.target.value, 10);
                onUpdate({ limit: isNaN(v) ? undefined : v });
              }}
              placeholder="No limit"
              width={16}
            />
          </InlineField>
          <InlineField label="Offset" labelWidth={LABEL_WIDTH}>
            <Input
              type="number"
              value={query.offset ?? ''}
              onChange={(e: ChangeEvent<HTMLInputElement>) => {
                const v = parseInt(e.target.value, 10);
                onUpdate({ offset: isNaN(v) ? undefined : v });
              }}
              placeholder="0"
              width={16}
            />
          </InlineField>
        </InlineFieldRow>
      </Collapse>
    </div>
  );
}

// --- Inline filter editor ---

interface FilterEditorProps {
  conditions: WhereCondition[];
  onChange: (conditions: WhereCondition[]) => void;
  onRunQuery: () => void;
}

const OPERATOR_OPTIONS = [
  { label: '=', value: 'eq' as const },
  { label: '!=', value: 'neq' as const },
  { label: '>', value: 'gt' as const },
  { label: '>=', value: 'gte' as const },
  { label: '<', value: 'lt' as const },
  { label: '<=', value: 'lte' as const },
];

function FilterEditor({ conditions, onChange, onRunQuery }: FilterEditorProps) {
  const styles = useStyles2(getStyles);

  const addCondition = () => {
    onChange([...conditions, { column: '', operator: 'eq', value: '' }]);
  };

  const updateCondition = (index: number, patch: Partial<WhereCondition>) => {
    const next = [...conditions];
    next[index] = { ...next[index], ...patch };
    onChange(next);
  };

  const removeCondition = (index: number) => {
    const next = [...conditions];
    next.splice(index, 1);
    onChange(next);
    onRunQuery();
  };

  return (
    <div className={styles.filterContainer}>
      {conditions.map((condition, index) => (
        <InlineFieldRow key={index}>
          <InlineField label="Column" labelWidth={10}>
            <Input
              value={condition.column}
              onChange={(e: ChangeEvent<HTMLInputElement>) =>
                updateCondition(index, { column: e.target.value })
              }
              placeholder="column name"
              width={16}
            />
          </InlineField>
          <InlineField label="Op" labelWidth={6}>
            <Combobox
              options={OPERATOR_OPTIONS}
              value={condition.operator}
              onChange={(option) => updateCondition(index, { operator: option.value as WhereCondition['operator'] })}
              width={10}
            />
          </InlineField>
          <InlineField label="Value" labelWidth={8}>
            <Input
              value={String(condition.value)}
              onChange={(e: ChangeEvent<HTMLInputElement>) => {
                const v = e.target.value;
                const num = Number(v);
                updateCondition(index, { value: isNaN(num) ? v : num });
              }}
              placeholder="value"
              width={16}
            />
          </InlineField>
          <button className={styles.removeButton} onClick={() => removeCondition(index)} type="button">
            &times;
          </button>
        </InlineFieldRow>
      ))}
      <button className={styles.addButton} onClick={addCondition} type="button">
        + Add filter
      </button>
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
    filterContainer: css({
      display: 'flex',
      flexDirection: 'column',
      gap: theme.spacing(0.25),
      padding: `${theme.spacing(0.5)} 0`,
    }),
    removeButton: css({
      background: 'none',
      border: 'none',
      cursor: 'pointer',
      color: theme.colors.text.secondary,
      fontSize: '18px',
      lineHeight: 1,
      padding: '4px 8px',
      '&:hover': {
        color: theme.colors.error.text,
      },
    }),
    addButton: css({
      background: 'none',
      border: `1px dashed ${theme.colors.border.weak}`,
      borderRadius: theme.shape.radius.default,
      cursor: 'pointer',
      color: theme.colors.text.secondary,
      fontSize: theme.typography.bodySmall.fontSize,
      padding: `${theme.spacing(0.5)} ${theme.spacing(1)}`,
      alignSelf: 'flex-start',
      '&:hover': {
        color: theme.colors.text.primary,
        borderColor: theme.colors.border.medium,
      },
    }),
  };
}

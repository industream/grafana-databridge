import React, { ChangeEvent, useState } from 'react';
import { css } from '@emotion/css';
import { Collapse, Combobox, InlineField, InlineFieldRow, Input, RadioButtonGroup, useStyles2 } from '@grafana/ui';
import { GrafanaTheme2 } from '@grafana/data';

import {
  DataBridgeQuery,
  FilterDefinition,
  FilterGroup,
  FilterCondition,
  ComparisonOperator,
  LogicalOperator,
  TimeWindowUnit,
  isFilterGroup,
} from '../types';

interface QueryOptionsProps {
  query: DataBridgeQuery;
  onUpdate: (patch: Partial<DataBridgeQuery>) => void;
  onUpdateAndRun: (patch: Partial<DataBridgeQuery>) => void;
  isMultiDataset?: boolean;
  optimizeDisplay?: boolean;
}

const TIME_WINDOW_INTERVAL_OPTIONS = [
  { label: 'Auto', value: '0' },
  { label: '1', value: '1' },
  { label: '5', value: '5' },
  { label: '10', value: '10' },
  { label: '15', value: '15' },
  { label: '30', value: '30' },
];

const TIME_WINDOW_UNIT_OPTIONS = [
  { label: 'seconds', value: 's' },
  { label: 'minutes', value: 'm' },
  { label: 'hours', value: 'h' },
  { label: 'days', value: 'd' },
];


function countConditions(filter?: FilterDefinition): number {
  if (!filter) {
    return 0;
  }
  if (isFilterGroup(filter)) {
    return filter.conditions.reduce((acc, c) => acc + countConditions(c), 0);
  }
  return 1;
}

export function QueryOptions({ query, onUpdate, onUpdateAndRun, isMultiDataset, optimizeDisplay }: QueryOptionsProps) {
  const styles = useStyles2(getStyles);

  const [isFiltersOpen, setIsFiltersOpen] = useState(false);

  const filterCount = countConditions(query.where);
  const filterSummary = isMultiDataset
    ? 'disabled (multiple datasets)'
    : filterCount > 0 ? `${filterCount} filter(s)` : '(none)';

  return (
    <div className={styles.container}>
      {/* Time Window — visible only when aggregation is active */}
      {optimizeDisplay && (
        <InlineFieldRow>
          <InlineField label="Time Window" labelWidth={16} tooltip="Bucket size for aggregation. Auto computes from time range and panel width.">
            <Combobox
              options={TIME_WINDOW_INTERVAL_OPTIONS}
              value={String(query.timeWindowInterval ?? 0)}
              onChange={(option) => {
                const interval = Number(option.value);
                if (interval === 0) {
                  onUpdateAndRun({ timeWindowInterval: 0, timeWindowUnit: undefined });
                } else {
                  onUpdateAndRun({ timeWindowInterval: interval, timeWindowUnit: query.timeWindowUnit ?? 's' });
                }
              }}
              width={10}
            />
          </InlineField>
          {(query.timeWindowInterval ?? 0) > 0 && (
            <InlineField>
              <Combobox
                options={TIME_WINDOW_UNIT_OPTIONS}
                value={query.timeWindowUnit ?? 's'}
                onChange={(option) => onUpdateAndRun({ timeWindowUnit: option.value as TimeWindowUnit })}
                width={14}
              />
            </InlineField>
          )}
        </InlineFieldRow>
      )}

      {/* Filters (WHERE) — collapsible, disabled when tags span multiple datasets */}
      <div className={isMultiDataset ? styles.disabledSection : undefined}>
        <Collapse
          label={`Filters (WHERE): ${filterSummary}`}
          isOpen={isMultiDataset ? false : isFiltersOpen}
          onToggle={() => {
            if (!isMultiDataset) {
              setIsFiltersOpen(!isFiltersOpen);
            }
          }}
        >
          <FilterGroupEditor
            group={ensureGroup(query.where)}
            isRoot
            onChange={(where) => onUpdate({ where })}
            onRunQuery={() => onUpdateAndRun({})}
          />
        </Collapse>
      </div>

    </div>
  );
}

// --- Nested filter editor ---

function ensureGroup(filter?: FilterDefinition): FilterGroup {
  if (!filter) {
    return { operator: 'and', conditions: [] };
  }
  if (isFilterGroup(filter)) {
    return filter;
  }
  return { operator: 'and', conditions: [filter] };
}

const OPERATOR_OPTIONS = [
  { label: '=', value: 'eq' as const },
  { label: '!=', value: 'neq' as const },
  { label: '>', value: 'gt' as const },
  { label: '>=', value: 'gte' as const },
  { label: '<', value: 'lt' as const },
  { label: '<=', value: 'lte' as const },
];

const COMBINATOR_OPTIONS = [
  { label: 'AND', value: 'and' as const },
  { label: 'OR', value: 'or' as const },
];

interface FilterGroupEditorProps {
  group: FilterGroup;
  isRoot?: boolean;
  onChange: (group: FilterGroup) => void;
  onRunQuery: () => void;
  onRemove?: () => void;
}

function FilterGroupEditor({ group, isRoot, onChange, onRunQuery, onRemove }: FilterGroupEditorProps) {
  const styles = useStyles2(getStyles);

  const updateOperator = (op: LogicalOperator) => {
    onChange({ ...group, operator: op });
  };

  const updateCondition = (index: number, updated: FilterDefinition) => {
    const next = [...group.conditions];
    next[index] = updated;
    onChange({ ...group, conditions: next });
  };

  const removeCondition = (index: number) => {
    const next = [...group.conditions];
    next.splice(index, 1);
    onChange({ ...group, conditions: next });
    onRunQuery();
  };

  const addCondition = () => {
    onChange({
      ...group,
      conditions: [...group.conditions, { column: '', operator: 'eq' as ComparisonOperator, value: '' }],
    });
  };

  const addGroup = () => {
    const newGroup: FilterGroup = { operator: group.operator === 'and' ? 'or' : 'and', conditions: [] };
    onChange({ ...group, conditions: [...group.conditions, newGroup] });
  };

  return (
    <div className={isRoot ? styles.filterContainer : styles.nestedGroup}>
      <div className={styles.groupHeader}>
        <RadioButtonGroup
          options={COMBINATOR_OPTIONS}
          value={group.operator}
          onChange={(v) => updateOperator(v as LogicalOperator)}
          size="sm"
        />
        {!isRoot && onRemove && (
          <button className={styles.removeButton} onClick={onRemove} type="button" title="Remove group">
            &times;
          </button>
        )}
      </div>

      {group.conditions.map((condition, index) => (
        <div key={index} className={styles.conditionRow}>
          {isFilterGroup(condition) ? (
            <FilterGroupEditor
              group={condition}
              onChange={(updated) => updateCondition(index, updated)}
              onRunQuery={onRunQuery}
              onRemove={() => removeCondition(index)}
            />
          ) : (
            <ConditionEditor
              condition={condition}
              onChange={(updated) => updateCondition(index, updated)}
              onRemove={() => removeCondition(index)}
            />
          )}
        </div>
      ))}

      <div className={styles.buttonRow}>
        <button className={styles.addButton} onClick={addCondition} type="button">
          + Condition
        </button>
        <button className={styles.addButton} onClick={addGroup} type="button">
          + Group
        </button>
      </div>
    </div>
  );
}

interface ConditionEditorProps {
  condition: FilterCondition;
  onChange: (condition: FilterCondition) => void;
  onRemove: () => void;
}

function ConditionEditor({ condition, onChange, onRemove }: ConditionEditorProps) {
  const styles = useStyles2(getStyles);

  return (
    <InlineFieldRow>
      <InlineField label="Column" labelWidth={10}>
        <Input
          value={condition.column}
          onChange={(e: ChangeEvent<HTMLInputElement>) =>
            onChange({ ...condition, column: e.target.value })
          }
          placeholder="column name"
          width={16}
        />
      </InlineField>
      <InlineField label="Op" labelWidth={6}>
        <Combobox
          options={OPERATOR_OPTIONS}
          value={condition.operator}
          onChange={(option) => onChange({ ...condition, operator: option.value as ComparisonOperator })}
          width={10}
        />
      </InlineField>
      <InlineField label="Value" labelWidth={8}>
        <Input
          value={String(condition.value)}
          onChange={(e: ChangeEvent<HTMLInputElement>) => {
            const v = e.target.value;
            const num = Number(v);
            onChange({ ...condition, value: isNaN(num) ? v : num });
          }}
          placeholder="value"
          width={16}
        />
      </InlineField>
      <button className={styles.removeButton} onClick={onRemove} type="button">
        &times;
      </button>
    </InlineFieldRow>
  );
}

function getStyles(theme: GrafanaTheme2) {
  return {
    container: css({
      display: 'flex',
      flexDirection: 'column',
      gap: theme.spacing(0.25),
    }),
    disabledSection: css({
      opacity: 0.5,
      pointerEvents: 'none',
    }),
    filterContainer: css({
      display: 'flex',
      flexDirection: 'column',
      gap: theme.spacing(0.5),
      padding: `${theme.spacing(0.5)} 0`,
    }),
    nestedGroup: css({
      display: 'flex',
      flexDirection: 'column',
      gap: theme.spacing(0.5),
      padding: theme.spacing(1),
      borderLeft: `2px solid ${theme.colors.primary.border}`,
      borderRadius: theme.shape.radius.default,
      background: theme.colors.background.secondary,
    }),
    groupHeader: css({
      display: 'flex',
      alignItems: 'center',
      gap: theme.spacing(1),
    }),
    conditionRow: css({
      display: 'flex',
      flexDirection: 'column',
    }),
    buttonRow: css({
      display: 'flex',
      gap: theme.spacing(0.5),
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
      '&:hover': {
        color: theme.colors.text.primary,
        borderColor: theme.colors.border.medium,
      },
    }),
  };
}

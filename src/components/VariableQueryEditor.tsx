import React, { ChangeEvent, useCallback, useEffect, useMemo, useState } from 'react';
import { Combobox, InlineField, InlineFieldRow, Input } from '@grafana/ui';
import { QueryEditorProps } from '@grafana/data';

import { DataSource } from '../datasource';
import { DataBridgeOptions, DataBridgeQuery, SourceConnection } from '../types';

export type VariableQueryType = 'connections' | 'databases' | 'datasets' | 'entries' | 'labels';

export interface VariableQuery {
  type: VariableQueryType;
  connectionId?: string;
  databaseName?: string;
  label?: string;
  search?: string;
}

const TYPE_OPTIONS = [
  { label: 'Connections', value: 'connections' },
  { label: 'Databases', value: 'databases' },
  { label: 'Datasets', value: 'datasets' },
  { label: 'Catalog Entries', value: 'entries' },
  { label: 'Labels', value: 'labels' },
];

const LABEL_WIDTH = 14;

type Props = QueryEditorProps<DataSource, DataBridgeQuery, DataBridgeOptions>;

export function VariableQueryEditor({ query, onChange, datasource }: Props) {
  const variableQuery: VariableQuery = useMemo(() => {
    const raw = (query as DataBridgeQuery & { variableQuery?: VariableQuery }).variableQuery;
    return raw ?? { type: 'connections' };
  }, [query]);

  const [connections, setConnections] = useState<SourceConnection[]>([]);

  useEffect(() => {
    let cancelled = false;
    datasource.getConnections().then((data) => {
      if (!cancelled) { setConnections(data); }
    }).catch(console.error);
    return () => { cancelled = true; };
  }, [datasource]);

  const connectionOptions = useMemo(
    () => connections.map((c) => ({ label: c.name, value: c.id })),
    [connections]
  );

  const update = useCallback(
    (patch: Partial<VariableQuery>) => {
      const next = { ...variableQuery, ...patch };
      onChange({ ...query, variableQuery: next } as DataBridgeQuery);
    },
    [onChange, query, variableQuery]
  );

  return (
    <>
      <InlineFieldRow>
        <InlineField label="Variable Type" labelWidth={LABEL_WIDTH}>
          <Combobox
            options={TYPE_OPTIONS}
            value={variableQuery.type}
            onChange={(option) => update({ type: option.value as VariableQueryType })}
            width={24}
          />
        </InlineField>
      </InlineFieldRow>

      {(variableQuery.type === 'databases' || variableQuery.type === 'datasets') && (
        <InlineFieldRow>
          <InlineField label="Connection" labelWidth={LABEL_WIDTH}>
            <Combobox
              options={connectionOptions}
              value={variableQuery.connectionId ?? null}
              onChange={(option) => update({ connectionId: option?.value })}
              placeholder="Select connection..."
              isClearable
              width={24}
            />
          </InlineField>
        </InlineFieldRow>
      )}

      {variableQuery.type === 'datasets' && (
        <InlineFieldRow>
          <InlineField label="Database" labelWidth={LABEL_WIDTH}>
            <Input
              value={variableQuery.databaseName ?? ''}
              onChange={(e: ChangeEvent<HTMLInputElement>) =>
                update({ databaseName: e.target.value || undefined })
              }
              placeholder="Database name or $variable"
              width={24}
            />
          </InlineField>
        </InlineFieldRow>
      )}

      {variableQuery.type === 'entries' && (
        <InlineFieldRow>
          <InlineField label="Label Filter" labelWidth={LABEL_WIDTH}>
            <Input
              value={variableQuery.label ?? ''}
              onChange={(e: ChangeEvent<HTMLInputElement>) =>
                update({ label: e.target.value || undefined })
              }
              placeholder="e.g. analog"
              width={20}
            />
          </InlineField>
          <InlineField label="Search" labelWidth={LABEL_WIDTH}>
            <Input
              value={variableQuery.search ?? ''}
              onChange={(e: ChangeEvent<HTMLInputElement>) =>
                update({ search: e.target.value || undefined })
              }
              placeholder="Search text..."
              width={20}
            />
          </InlineField>
        </InlineFieldRow>
      )}
    </>
  );
}

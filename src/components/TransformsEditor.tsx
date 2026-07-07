import React, { useState } from 'react';
import { css } from '@emotion/css';
import { Checkbox, Collapse, Combobox, InlineField, InlineFieldRow, InlineSwitch, Input, useStyles2 } from '@grafana/ui';
import { GrafanaTheme2 } from '@grafana/data';

import {
  DataBridgeQuery,
  TransformKind,
  ROLLING_STATS_OPTIONS,
  FILL_METHOD_OPTIONS,
} from '../types';
import { getTransformParams, setTransformInPipeline } from '../utils/transforms';

interface TransformsEditorProps {
  query: DataBridgeQuery;
  onUpdateAndRun: (patch: Partial<DataBridgeQuery>) => void;
}

const LABEL_WIDTH = 18;

const AGGREGATION_OPTIONS = ['avg', 'sum', 'min', 'max', 'count', 'first', 'last'].map((v) => ({
  label: v,
  value: v,
}));

const FILL_OPTIONS = FILL_METHOD_OPTIONS.map((v) => ({ label: v, value: v }));

export function TransformsEditor({ query, onUpdateAndRun }: TransformsEditorProps) {
  const styles = useStyles2(getStyles);
  const [isOpen, setIsOpen] = useState(false);

  const transforms = query.transforms ?? [];
  const enabledCount = transforms.length;

  const setTransform = (kind: TransformKind, params: object | undefined) => {
    onUpdateAndRun({ transforms: setTransformInPipeline(transforms, kind, params) });
  };

  const toggle = (kind: TransformKind, enabled: boolean, defaults: object) => {
    setTransform(kind, enabled ? defaults : undefined);
  };

  const resample = getTransformParams(transforms, 'resample');
  const fill = getTransformParams(transforms, 'fill');
  const movingAverage = getTransformParams(transforms, 'movingAverage');
  const cumulativeSum = getTransformParams(transforms, 'cumulativeSum');
  const rollingStats = getTransformParams(transforms, 'rollingStats');

  const rollingSelected = (rollingStats?.stats as string[] | undefined) ?? [];

  return (
    <Collapse
      label={`Transforms: ${enabledCount > 0 ? `${enabledCount} enabled` : '(none)'}`}
      isOpen={isOpen}
      onToggle={() => setIsOpen(!isOpen)}
    >
      <div className={styles.section}>
        {/* Resample */}
        <InlineFieldRow>
          <InlineField label="Resample" labelWidth={LABEL_WIDTH} tooltip="Bucket by time and aggregate. Replaces the automatic Optimize-display downsampling when enabled.">
            <InlineSwitch
              value={resample !== undefined}
              onChange={() => toggle('resample', resample === undefined, { every: 'PT1M', aggregation: 'avg' })}
            />
          </InlineField>
          {resample !== undefined && (
            <>
              <InlineField label="Every" tooltip="ISO-8601 duration, e.g. PT1M, PT5M, PT1H.">
                <Input
                  width={12}
                  value={(resample.every as string) ?? ''}
                  placeholder="PT1M"
                  onBlur={(e) => setTransform('resample', { ...resample, every: e.currentTarget.value })}
                />
              </InlineField>
              <InlineField label="Aggregation">
                <Combobox
                  width={14}
                  options={AGGREGATION_OPTIONS}
                  value={(resample.aggregation as string) ?? 'avg'}
                  onChange={(o) => setTransform('resample', { ...resample, aggregation: o.value })}
                />
              </InlineField>
              <InlineField label="Create empty" tooltip="Emit empty buckets for gaps.">
                <InlineSwitch
                  value={Boolean(resample.createEmpty)}
                  onChange={(e) => setTransform('resample', { ...resample, createEmpty: e.currentTarget.checked })}
                />
              </InlineField>
            </>
          )}
        </InlineFieldRow>

        {/* Fill */}
        <InlineFieldRow>
          <InlineField label="Fill" labelWidth={LABEL_WIDTH} tooltip="Fill gaps in the series.">
            <InlineSwitch
              value={fill !== undefined}
              onChange={() => toggle('fill', fill === undefined, { method: 'linear' })}
            />
          </InlineField>
          {fill !== undefined && (
            <>
              <InlineField label="Method">
                <Combobox
                  width={14}
                  options={FILL_OPTIONS}
                  value={(fill.method as string) ?? 'linear'}
                  onChange={(o) => setTransform('fill', { ...fill, method: o.value })}
                />
              </InlineField>
              {fill.method === 'value' && (
                <InlineField label="Value">
                  <Input
                    width={10}
                    type="number"
                    value={fill.value !== undefined ? String(fill.value) : ''}
                    onBlur={(e) =>
                      setTransform('fill', {
                        ...fill,
                        value: e.currentTarget.value === '' ? undefined : Number(e.currentTarget.value),
                      })
                    }
                  />
                </InlineField>
              )}
              <InlineField label="Limit" tooltip="Max consecutive points to fill (0 = unlimited).">
                <Input
                  width={10}
                  type="number"
                  value={fill.limit !== undefined ? String(fill.limit) : ''}
                  onBlur={(e) =>
                    setTransform('fill', {
                      ...fill,
                      limit: e.currentTarget.value === '' ? undefined : Number(e.currentTarget.value),
                    })
                  }
                />
              </InlineField>
            </>
          )}
        </InlineFieldRow>

        {/* Moving average */}
        <InlineFieldRow>
          <InlineField label="Moving average" labelWidth={LABEL_WIDTH} tooltip="Sliding-window mean per numeric column.">
            <InlineSwitch
              value={movingAverage !== undefined}
              onChange={() => toggle('movingAverage', movingAverage === undefined, { window: 5 })}
            />
          </InlineField>
          {movingAverage !== undefined && (
            <InlineField label="Window">
              <Input
                width={10}
                type="number"
                value={String(movingAverage.window ?? 5)}
                onBlur={(e) => setTransform('movingAverage', { window: Number(e.currentTarget.value) || 1 })}
              />
            </InlineField>
          )}
        </InlineFieldRow>

        {/* Cumulative sum */}
        <InlineFieldRow>
          <InlineField label="Cumulative sum" labelWidth={LABEL_WIDTH} tooltip="Running total per numeric column.">
            <InlineSwitch
              value={cumulativeSum !== undefined}
              onChange={() => toggle('cumulativeSum', cumulativeSum === undefined, {})}
            />
          </InlineField>
        </InlineFieldRow>

        {/* Rolling stats */}
        <InlineFieldRow>
          <InlineField label="Rolling stats" labelWidth={LABEL_WIDTH} tooltip="Sliding-window statistics per numeric column.">
            <InlineSwitch
              value={rollingStats !== undefined}
              onChange={() => toggle('rollingStats', rollingStats === undefined, { window: 10 })}
            />
          </InlineField>
          {rollingStats !== undefined && (
            <InlineField label="Window">
              <Input
                width={10}
                type="number"
                value={String(rollingStats.window ?? 10)}
                onBlur={(e) => setTransform('rollingStats', { ...rollingStats, window: Number(e.currentTarget.value) || 1 })}
              />
            </InlineField>
          )}
        </InlineFieldRow>
        {rollingStats !== undefined && (
          <div className={styles.statsRow}>
            {ROLLING_STATS_OPTIONS.map((stat) => (
              <Checkbox
                key={stat}
                label={stat}
                value={rollingSelected.includes(stat)}
                onChange={(e) => {
                  const checked = e.currentTarget.checked;
                  const nextStats = checked
                    ? [...rollingSelected, stat]
                    : rollingSelected.filter((s) => s !== stat);
                  setTransform('rollingStats', {
                    ...rollingStats,
                    stats: nextStats.length > 0 ? nextStats : undefined,
                  });
                }}
              />
            ))}
          </div>
        )}
      </div>
    </Collapse>
  );
}

const getStyles = (theme: GrafanaTheme2) => ({
  section: css({
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(0.5),
    paddingTop: theme.spacing(0.5),
  }),
  statsRow: css({
    display: 'flex',
    flexWrap: 'wrap',
    gap: theme.spacing(2),
    paddingLeft: theme.spacing(19),
    paddingBottom: theme.spacing(1),
  }),
});

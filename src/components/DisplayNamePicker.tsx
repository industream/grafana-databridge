import React, { ChangeEvent } from 'react';
import { css } from '@emotion/css';
import { Combobox, InlineField, InlineFieldRow, Input, useStyles2 } from '@grafana/ui';
import { GrafanaTheme2 } from '@grafana/data';

import { CatalogEntry, DisplayNamePreset } from '../types';
import { resolveDisplayName } from '../hooks/useDisplayName';

interface DisplayNamePickerProps {
  preset: DisplayNamePreset;
  pattern: string;
  entries: CatalogEntry[];
  onPresetChange: (preset: DisplayNamePreset) => void;
  onPatternChange: (pattern: string) => void;
}

const PRESET_OPTIONS = [
  { label: 'Entry Name', value: 'entryName' as const },
  { label: 'Tag Level 1', value: 'tagLevel1' as const },
  { label: 'Description (EN)', value: 'descriptionEn' as const },
  { label: 'Description (DE)', value: 'descriptionDe' as const },
  { label: 'Asset Path', value: 'assetPath' as const },
  { label: 'Custom Pattern', value: 'custom' as const },
];

const LABEL_WIDTH = 14;

export function DisplayNamePicker({
  preset,
  pattern,
  entries,
  onPresetChange,
  onPatternChange,
}: DisplayNamePickerProps) {
  const styles = useStyles2(getStyles);

  const previewEntries = entries.slice(0, 3);
  const previews = previewEntries.map((entry) =>
    resolveDisplayName(entry, preset, pattern, { column: entry.name })
  );

  return (
    <div className={styles.container}>
      <InlineFieldRow>
        <InlineField label="Display" labelWidth={LABEL_WIDTH}>
          <Combobox
            options={PRESET_OPTIONS}
            value={preset}
            onChange={(option) => onPresetChange(option.value as DisplayNamePreset)}
            width={24}
          />
        </InlineField>
      </InlineFieldRow>

      {preset === 'custom' && (
        <InlineFieldRow>
          <InlineField
            label="Pattern"
            labelWidth={LABEL_WIDTH}
            tooltip="Variables: {name}, {tagLevel1}, {description.en-US}, {unit}, {label}, {aggregation}, {asset.path}"
          >
            <Input
              value={pattern}
              onChange={(event: ChangeEvent<HTMLInputElement>) => onPatternChange(event.target.value)}
              placeholder="{name} [{tagLevel1}]"
              width={40}
            />
          </InlineField>
        </InlineFieldRow>
      )}

      {/* Live preview */}
      {previews.length > 0 && (
        <div className={styles.preview}>
          <span className={styles.previewLabel}>Preview:</span>
          {previews.map((name, i) => (
            <span key={previewEntries[i].id} className={styles.previewItem}>
              {name}
            </span>
          ))}
          {entries.length > 3 && (
            <span className={styles.previewMore}>+{entries.length - 3} more</span>
          )}
        </div>
      )}
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
    preview: css({
      display: 'flex',
      alignItems: 'center',
      flexWrap: 'wrap',
      gap: theme.spacing(0.5),
      padding: `${theme.spacing(0.25)} ${theme.spacing(0.5)}`,
      marginLeft: `${LABEL_WIDTH * 8}px`,
    }),
    previewLabel: css({
      fontSize: theme.typography.bodySmall.fontSize,
      color: theme.colors.text.secondary,
      flexShrink: 0,
    }),
    previewItem: css({
      fontSize: theme.typography.bodySmall.fontSize,
      color: theme.colors.text.primary,
      backgroundColor: theme.colors.background.secondary,
      padding: `0 ${theme.spacing(0.5)}`,
      borderRadius: theme.shape.radius.default,
    }),
    previewMore: css({
      fontSize: theme.typography.bodySmall.fontSize,
      color: theme.colors.text.disabled,
    }),
  };
}

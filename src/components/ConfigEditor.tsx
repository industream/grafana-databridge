import React, { ChangeEvent } from 'react';
import { FieldSet, InlineField, InlineFieldRow, Input, SecretInput, Combobox } from '@grafana/ui';
import { DataSourcePluginOptionsEditorProps } from '@grafana/data';

import { DataBridgeOptions, DataBridgeSecureJsonData, DisplayNamePreset } from '../types';

interface Props extends DataSourcePluginOptionsEditorProps<DataBridgeOptions, DataBridgeSecureJsonData> {}

const DISPLAY_NAME_OPTIONS = [
  { label: 'Entry Name', value: 'entryName' as const },
  { label: 'Tag Level 1', value: 'tagLevel1' as const },
  { label: 'Description (EN)', value: 'descriptionEn' as const },
  { label: 'Description (DE)', value: 'descriptionDe' as const },
  { label: 'Asset Path', value: 'assetPath' as const },
  { label: 'Custom Pattern', value: 'custom' as const },
];

const LABEL_WIDTH = 24;
const INPUT_WIDTH = 40;

export function ConfigEditor({ options, onOptionsChange }: Props) {
  const { jsonData, secureJsonFields, secureJsonData } = options;

  const updateJsonData = <K extends keyof DataBridgeOptions>(key: K, value: DataBridgeOptions[K]) => {
    onOptionsChange({
      ...options,
      jsonData: { ...jsonData, [key]: value },
    });
  };

  const onInputChange = (key: keyof DataBridgeOptions) => (event: ChangeEvent<HTMLInputElement>) => {
    updateJsonData(key, event.target.value);
  };

  const onNumberChange = (key: keyof DataBridgeOptions) => (event: ChangeEvent<HTMLInputElement>) => {
    const value = parseInt(event.target.value, 10);
    updateJsonData(key, isNaN(value) ? undefined : value);
  };

  const onApiKeyChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      secureJsonData: { apiKey: event.target.value },
    });
  };

  const onResetApiKey = () => {
    onOptionsChange({
      ...options,
      secureJsonFields: { ...secureJsonFields, apiKey: false },
      secureJsonData: { ...secureJsonData, apiKey: '' },
    });
  };

  return (
    <>
      <FieldSet label="Connection">
        <InlineField label="DataCatalog API URL" labelWidth={LABEL_WIDTH} tooltip="URL of the DataCatalog API. DataBridge URLs are resolved automatically from source connections.">
          <Input
            id="config-datacatalog-url"
            onChange={onInputChange('dataCatalogApiUrl')}
            value={jsonData.dataCatalogApiUrl ?? ''}
            placeholder="http://localhost:8010"
            width={INPUT_WIDTH}
          />
        </InlineField>
        <InlineField label="API Key" labelWidth={LABEL_WIDTH} tooltip="Encrypted credential for API authentication">
          <SecretInput
            id="config-api-key"
            isConfigured={secureJsonFields.apiKey ?? false}
            value={secureJsonData?.apiKey ?? ''}
            placeholder="Enter your API key"
            width={INPUT_WIDTH}
            onReset={onResetApiKey}
            onChange={onApiKeyChange}
          />
        </InlineField>
      </FieldSet>

      <FieldSet label="Display Defaults">
        <InlineFieldRow>
          <InlineField label="Default Display Name" labelWidth={LABEL_WIDTH} tooltip="How tag names appear in panels">
            <Combobox
              id="config-display-name"
              options={DISPLAY_NAME_OPTIONS}
              value={jsonData.defaultDisplayNamePreset ?? 'entryName'}
              onChange={(option) => updateJsonData('defaultDisplayNamePreset', option.value as DisplayNamePreset)}
              width={INPUT_WIDTH}
            />
          </InlineField>
        </InlineFieldRow>
      </FieldSet>

      <FieldSet label="Safety Limits">
        <InlineField
          label="Max Raw Rows"
          labelWidth={LABEL_WIDTH}
          tooltip="Auto-injected LIMIT for raw queries (warning threshold)"
        >
          <Input
            id="config-max-raw-rows"
            type="number"
            onChange={onNumberChange('maxRawRows')}
            value={jsonData.maxRawRows ?? 50000}
            width={INPUT_WIDTH}
          />
        </InlineField>
        <InlineField
          label="Hard Limit (No Bypass)"
          labelWidth={LABEL_WIDTH}
          tooltip="Absolute maximum rows - queries above this are blocked"
        >
          <Input
            id="config-hard-limit"
            type="number"
            onChange={onNumberChange('hardLimitRows')}
            value={jsonData.hardLimitRows ?? 1000000}
            width={INPUT_WIDTH}
          />
        </InlineField>
        <InlineField label="Cache TTL (seconds)" labelWidth={LABEL_WIDTH} tooltip="How long to cache catalog/schema data">
          <Input
            id="config-cache-ttl"
            type="number"
            onChange={onNumberChange('cacheTtlSeconds')}
            value={jsonData.cacheTtlSeconds ?? 300}
            width={INPUT_WIDTH}
          />
        </InlineField>
      </FieldSet>
    </>
  );
}

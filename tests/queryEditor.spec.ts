import { test, expect } from '@grafana/plugin-e2e';

test('smoke: should render query editor with mode selector', async ({ panelEditPage, readProvisionedDataSource }) => {
  const ds = await readProvisionedDataSource({ fileName: 'datasources.yml' });
  await panelEditPage.datasource.set(ds.name);
  await expect(panelEditPage.getQueryEditorRow('A').getByRole('radio', { name: 'DataCatalog' })).toBeVisible();
  await expect(panelEditPage.getQueryEditorRow('A').getByRole('radio', { name: 'Raw' })).toBeVisible();
});

test('should show asset tree in DataCatalog mode', async ({ panelEditPage, readProvisionedDataSource }) => {
  const ds = await readProvisionedDataSource({ fileName: 'datasources.yml' });
  await panelEditPage.datasource.set(ds.name);
  // DataCatalog is the default mode — search input should be visible
  await expect(panelEditPage.getQueryEditorRow('A').getByPlaceholder('Search tags...')).toBeVisible();
});

test('should show connection dropdown in Raw mode', async ({ panelEditPage, readProvisionedDataSource }) => {
  const ds = await readProvisionedDataSource({ fileName: 'datasources.yml' });
  await panelEditPage.datasource.set(ds.name);
  // Switch to Raw mode
  await panelEditPage.getQueryEditorRow('A').getByRole('radio', { name: 'Raw' }).click();
  await expect(panelEditPage.getQueryEditorRow('A').getByText('Connection')).toBeVisible();
});

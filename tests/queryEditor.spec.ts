import { test, expect } from '@grafana/plugin-e2e';

test('smoke: should render query editor with mode selector', async ({ panelEditPage, readProvisionedDataSource }) => {
  const ds = await readProvisionedDataSource({ fileName: 'datasources.yml' });
  await panelEditPage.datasource.set(ds.name);
  await expect(panelEditPage.getQueryEditorRow('A').getByRole('radio', { name: 'DataCatalog' })).toBeVisible();
  await expect(panelEditPage.getQueryEditorRow('A').getByRole('radio', { name: 'Raw' })).toBeVisible();
});

test('should show strategy selector', async ({ panelEditPage, readProvisionedDataSource }) => {
  const ds = await readProvisionedDataSource({ fileName: 'datasources.yml' });
  await panelEditPage.datasource.set(ds.name);
  await expect(panelEditPage.getQueryEditorRow('A').getByRole('radio', { name: 'Time Series' })).toBeVisible();
  await expect(panelEditPage.getQueryEditorRow('A').getByRole('radio', { name: 'Table' })).toBeVisible();
});

test('should switch to Raw mode', async ({ panelEditPage, readProvisionedDataSource }) => {
  const ds = await readProvisionedDataSource({ fileName: 'datasources.yml' });
  await panelEditPage.datasource.set(ds.name);
  await panelEditPage.getQueryEditorRow('A').getByRole('radio', { name: 'Raw' }).click();
  await expect(panelEditPage.getQueryEditorRow('A').getByText('Connection')).toBeVisible();
});

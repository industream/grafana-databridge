import { test, expect } from '@grafana/plugin-e2e';

test('smoke: should render config editor', async ({ createDataSourceConfigPage, readProvisionedDataSource, page }) => {
  const ds = await readProvisionedDataSource({ fileName: 'datasources.yml' });
  await createDataSourceConfigPage({ type: ds.type });
  await expect(page.getByLabel('DataCatalog API URL')).toBeVisible();
  await expect(page.getByLabel('API Key')).toBeVisible();
});

test('"Save & test" should fail when no backend is available', async ({
  createDataSourceConfigPage,
  readProvisionedDataSource,
}) => {
  const ds = await readProvisionedDataSource({ fileName: 'datasources.yml' });
  const configPage = await createDataSourceConfigPage({ type: ds.type });
  // No real backend in CI — save & test should fail
  await expect(configPage.saveAndTest()).not.toBeOK();
});

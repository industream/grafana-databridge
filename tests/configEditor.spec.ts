import { test, expect } from '@grafana/plugin-e2e';
import { DataBridgeOptions, DataBridgeSecureJsonData } from '../src/types';

test('smoke: should render config editor', async ({ createDataSourceConfigPage, readProvisionedDataSource, page }) => {
  const ds = await readProvisionedDataSource({ fileName: 'datasources.yml' });
  await createDataSourceConfigPage({ type: ds.type });
  await expect(page.getByLabel('DataCatalog API URL')).toBeVisible();
});

test('"Save & test" should be successful when configuration is valid', async ({
  createDataSourceConfigPage,
  readProvisionedDataSource,
  page,
}) => {
  const ds = await readProvisionedDataSource<DataBridgeOptions, DataBridgeSecureJsonData>({ fileName: 'datasources.yml' });
  const configPage = await createDataSourceConfigPage({ type: ds.type });
  await page.getByLabel('DataCatalog API URL').fill(ds.jsonData.dataCatalogApiUrl ?? 'http://localhost:8010');
  await page.getByLabel('API Key').fill(ds.secureJsonData?.apiKey ?? 'test-key');
  await expect(configPage.saveAndTest()).toBeOK();
});

test('"Save & test" should fail when configuration is invalid', async ({
  createDataSourceConfigPage,
  readProvisionedDataSource,
  page,
}) => {
  const ds = await readProvisionedDataSource<DataBridgeOptions, DataBridgeSecureJsonData>({ fileName: 'datasources.yml' });
  const configPage = await createDataSourceConfigPage({ type: ds.type });
  // Leave DataCatalog URL empty — health check should fail
  await page.getByLabel('DataCatalog API URL').fill('');
  await expect(configPage.saveAndTest()).not.toBeOK();
});

import { DataSourcePlugin } from '@grafana/data';

import { DataSource } from './datasource';
import { ConfigEditor } from './components/ConfigEditor';
import { QueryEditor } from './components/QueryEditor';
import { DataBridgeQuery, DataBridgeOptions } from './types';

export const plugin = new DataSourcePlugin<DataSource, DataBridgeQuery, DataBridgeOptions>(DataSource)
  .setConfigEditor(ConfigEditor)
  .setQueryEditor(QueryEditor);

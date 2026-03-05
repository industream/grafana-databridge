package plugin

import (
	"context"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"

	"github.com/industream/industream-data-bridge/pkg/cache"
	"github.com/industream/industream-data-bridge/pkg/datacatalog"
	"github.com/industream/industream-data-bridge/pkg/databridge"
	"github.com/industream/industream-data-bridge/pkg/models"
)

var (
	_ backend.QueryDataHandler    = (*Datasource)(nil)
	_ backend.CheckHealthHandler  = (*Datasource)(nil)
	_ backend.CallResourceHandler = (*Datasource)(nil)
	_ instancemgmt.InstanceDisposer = (*Datasource)(nil)
)

// Datasource is the main plugin instance, created per datasource configuration.
type Datasource struct {
	settings        *models.PluginSettings
	catalogClient   *datacatalog.Client
	connectionCache *cache.Store[[]datacatalog.SourceConnection]
	entryCache      *cache.Store[[]datacatalog.CatalogEntry]
	labelCache      *cache.Store[[]datacatalog.Label]
	assetCache      *cache.Store[[]datacatalog.AssetDictionary]
	logger          log.Logger
}

// NewDatasource creates a new datasource instance from Grafana settings.
func NewDatasource(_ context.Context, settings backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	pluginSettings, err := models.LoadPluginSettings(settings)
	if err != nil {
		return nil, err
	}

	ttl := time.Duration(pluginSettings.CacheTtlSeconds) * time.Second

	ds := &Datasource{
		settings:        pluginSettings,
		logger:          log.DefaultLogger,
		connectionCache: cache.NewStore[[]datacatalog.SourceConnection](ttl),
		entryCache:      cache.NewStore[[]datacatalog.CatalogEntry](ttl),
		labelCache:      cache.NewStore[[]datacatalog.Label](ttl),
		assetCache:      cache.NewStore[[]datacatalog.AssetDictionary](ttl),
	}

	if pluginSettings.DataCatalogApiUrl != "" {
		ds.catalogClient = datacatalog.NewClient(pluginSettings.DataCatalogApiUrl)
	}

	return ds, nil
}

// Dispose cleans up resources when the datasource instance is recreated.
func (d *Datasource) Dispose() {
	d.connectionCache.Clear()
	d.entryCache.Clear()
	d.labelCache.Clear()
	d.assetCache.Clear()
}

// dataBridgeClient creates a DataBridge client for the given URL.
func (d *Datasource) dataBridgeClient(url string) *databridge.Client {
	if url == "" {
		url = d.settings.DataBridgeApiUrl
	}
	return databridge.NewClient(url)
}

// resolveConnectionUrl looks up a source connection by ID and returns its URL.
func (d *Datasource) resolveConnectionUrl(ctx context.Context, connectionId string) (string, error) {
	connections, err := d.getConnections(ctx)
	if err != nil {
		return "", err
	}
	for _, c := range connections {
		if c.ID == connectionId {
			return c.URL, nil
		}
	}
	return d.settings.DataBridgeApiUrl, nil
}

// getConnections returns cached source connections, fetching from DataCatalog if needed.
func (d *Datasource) getConnections(ctx context.Context) ([]datacatalog.SourceConnection, error) {
	if conns, ok := d.connectionCache.Get("all"); ok {
		return conns, nil
	}

	if d.catalogClient == nil {
		return nil, nil
	}

	conns, err := d.catalogClient.ListConnections(ctx)
	if err != nil {
		return nil, err
	}
	d.connectionCache.Set("all", conns)
	return conns, nil
}

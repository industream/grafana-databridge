package plugin

import (
	"context"
	"fmt"
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
	_ backend.QueryDataHandler      = (*Datasource)(nil)
	_ backend.CheckHealthHandler    = (*Datasource)(nil)
	_ backend.CallResourceHandler   = (*Datasource)(nil)
	_ backend.StreamHandler         = (*Datasource)(nil)
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
	assetPathCache  *cache.Store[map[string]string]
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
		assetPathCache:  cache.NewStore[map[string]string](ttl),
	}

	if pluginSettings.DataCatalogApiUrl != "" {
		ds.catalogClient = datacatalog.NewClient(pluginSettings.DataCatalogApiUrl, pluginSettings.Secrets.ApiKey)
	}

	return ds, nil
}

// Dispose cleans up resources when the datasource instance is recreated.
func (d *Datasource) Dispose() {
	d.connectionCache.Clear()
	d.entryCache.Clear()
	d.labelCache.Clear()
	d.assetCache.Clear()
	d.assetPathCache.Clear()
}

// dataBridgeClient creates a DataBridge client for the given URL.
func (d *Datasource) dataBridgeClient(url string) *databridge.Client {
	if url == "" {
		url = d.settings.DataBridgeApiUrl
	}
	return databridge.NewClient(url)
}

// resolveConnectionUrl returns the DataBridge URL for a given connection ID.
// If a connection ID is provided and the DataCatalog is configured, it looks up the URL
// from the source connection. Otherwise falls back to the configured DataBridgeApiUrl.
func (d *Datasource) resolveConnectionUrl(ctx context.Context, connectionId string) (string, error) {
	if connectionId != "" && d.catalogClient != nil {
		conns, err := d.getConnections(ctx)
		if err != nil {
			return "", fmt.Errorf("fetch connections: %w", err)
		}
		for _, c := range conns {
			if c.ID == connectionId {
				if c.URL != "" {
					return c.URL, nil
				}
				break
			}
		}
	}
	if d.settings.DataBridgeApiUrl == "" {
		return "", fmt.Errorf("no DataBridge URL configured and connection %q not found", connectionId)
	}
	return d.settings.DataBridgeApiUrl, nil
}

// remapToDataBridge replaces non-DataBridge entries with their DataBridge counterparts (matched by name).
// Returns the updated entries and a map of old ID → new ID for entries that were remapped.
func (d *Datasource) remapToDataBridge(ctx context.Context, entries []datacatalog.CatalogEntry) ([]datacatalog.CatalogEntry, map[string]string) {
	// Check if any entries need remapping
	needsRemap := false
	for _, e := range entries {
		if !e.IsDataBridgeEntry() {
			needsRemap = true
			break
		}
	}
	if !needsRemap {
		return entries, nil
	}

	// Fetch all entries to find DataBridge counterparts by name
	allEntries, err := d.catalogClient.ListEntries(ctx, "", "")
	if err != nil {
		d.logger.Warn("Failed to fetch entries for remap", "error", err)
		return entries, nil
	}

	// Index DataBridge entries by name
	dbByName := make(map[string]*datacatalog.CatalogEntry)
	for i := range allEntries {
		if allEntries[i].IsDataBridgeEntry() {
			dbByName[allEntries[i].Name] = &allEntries[i]
		}
	}

	remapped := make(map[string]string)
	result := make([]datacatalog.CatalogEntry, 0, len(entries))
	for _, e := range entries {
		if e.IsDataBridgeEntry() {
			result = append(result, e)
			continue
		}
		// Find DataBridge counterpart by name
		if dbEntry, ok := dbByName[e.Name]; ok {
			remapped[e.ID] = dbEntry.ID
			result = append(result, *dbEntry)
			d.logger.Info("Remapped non-DataBridge entry to DataBridge", "name", e.Name, "oldId", e.ID, "newId", dbEntry.ID)
		} else {
			d.logger.Warn("No DataBridge counterpart found", "name", e.Name, "id", e.ID)
		}
	}
	return result, remapped
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

package plugin

import (
	"context"
	"fmt"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"golang.org/x/sync/singleflight"

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

	// sf collapses concurrent cold-cache misses for the shared caches
	// (connections, all-entries, asset tree) into a single in-flight fetch,
	// avoiding a thundering herd when many panels load at once.
	sf singleflight.Group
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
	allEntries, err := d.getAllEntries(ctx)
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

// getAllEntries returns all DataBridge catalog entries, served from entryCache.
// The underlying ListEntries call filters server-side to sourceTypes=DataBridge.
// This single cached fetch backs both remapToDataBridge and the column fallback.
func (d *Datasource) getAllEntries(ctx context.Context) ([]datacatalog.CatalogEntry, error) {
	if entries, ok := d.entryCache.Get("all"); ok {
		return entries, nil
	}
	if d.catalogClient == nil {
		return nil, nil
	}
	v, err, _ := d.sf.Do("allEntries", func() (interface{}, error) {
		// Re-check the cache inside the flight: another goroutine may have
		// populated it while this one was blocked acquiring the single flight.
		if entries, ok := d.entryCache.Get("all"); ok {
			return entries, nil
		}
		// Detach cancellation: this fetch is shared by every singleflight joiner,
		// so the first caller's cancellation must not abort it for the others.
		// The catalog client's own timeout bounds the call.
		entries, err := d.catalogClient.ListEntries(context.WithoutCancel(ctx), "", "")
		if err != nil {
			return nil, err
		}
		d.entryCache.Set("all", entries)
		return entries, nil
	})
	if err != nil {
		return nil, err
	}
	return v.([]datacatalog.CatalogEntry), nil
}

// getEntriesByIds resolves catalog entries for the given binding ids, serving
// from the cached all-entries listing first and only fetching the ids that are
// missing from the cache. This keeps per-panel queries off the DataCatalog API
// on the hot path while preserving GetEntriesByIds' binding-id match semantics
// (entries are keyed by CatalogEntry.ID, not the logical GetLogicalID()).
func (d *Datasource) getEntriesByIds(ctx context.Context, ids []string) ([]datacatalog.CatalogEntry, error) {
	if len(ids) == 0 || d.catalogClient == nil {
		return nil, nil
	}

	all, err := d.getAllEntries(ctx)
	if err != nil {
		// The all-entries listing is unavailable — fall back to a direct fetch so
		// the query path degrades to the previous behaviour rather than failing.
		return d.catalogClient.GetEntriesByIds(ctx, ids)
	}

	byID := make(map[string]*datacatalog.CatalogEntry, len(all))
	for i := range all {
		byID[all[i].ID] = &all[i]
	}

	result := make([]datacatalog.CatalogEntry, 0, len(ids))
	seen := make(map[string]bool, len(ids))
	var missing []string
	for _, id := range ids {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		if e, ok := byID[id]; ok {
			result = append(result, *e)
		} else {
			missing = append(missing, id)
		}
	}

	if len(missing) > 0 {
		fetched, err := d.catalogClient.GetEntriesByIds(ctx, missing)
		if err != nil {
			return nil, err
		}
		result = append(result, fetched...)
	}

	return result, nil
}

// dataBridgeEntriesByColumn builds an index of DataBridge entries keyed by their
// stable "column" source param. It is the cross-instance fallback: when a query's
// catalogEntryId (a per-instance Guid) does not resolve locally, the column key
// recovers the right DataBridge routing target.
//
// The index is built from the cached all-entries listing, which is already
// filtered server-side to sourceTypes=DataBridge — so we key purely on a
// non-empty column and do NOT gate on IsDataBridgeEntry() (the list payload does
// not reliably hydrate the nested sourceConnection.sourceType, which would
// otherwise wrongly empty the index). On a duplicate column the first entry in
// listing order wins deterministically and a collision warning is logged.
func (d *Datasource) dataBridgeEntriesByColumn(ctx context.Context) map[string]*datacatalog.CatalogEntry {
	all, err := d.getAllEntries(ctx)
	if err != nil {
		d.logger.Warn("Failed to fetch entries for column fallback", "error", err)
		return map[string]*datacatalog.CatalogEntry{}
	}

	idx := make(map[string]*datacatalog.CatalogEntry, len(all))
	for i := range all {
		e := &all[i]
		col := e.GetSourceParam("column")
		if col == "" {
			continue
		}
		if existing, dup := idx[col]; dup {
			d.logger.Warn("Column fallback collision: multiple DataBridge entries share a column; keeping first",
				"column", col,
				"keptDatabase", existing.GetSourceParam("database"),
				"keptDataset", existing.GetSourceParam("dataset"),
				"droppedDatabase", e.GetSourceParam("database"),
				"droppedDataset", e.GetSourceParam("dataset"),
			)
			continue
		}
		idx[col] = e
	}
	return idx
}

// getConnections returns cached source connections, fetching from DataCatalog if needed.
func (d *Datasource) getConnections(ctx context.Context) ([]datacatalog.SourceConnection, error) {
	if conns, ok := d.connectionCache.Get("all"); ok {
		return conns, nil
	}

	if d.catalogClient == nil {
		return nil, nil
	}

	v, err, _ := d.sf.Do("connections", func() (interface{}, error) {
		if conns, ok := d.connectionCache.Get("all"); ok {
			return conns, nil
		}
		// Detached: shared across singleflight joiners (see getAllEntries).
		conns, err := d.catalogClient.ListConnections(context.WithoutCancel(ctx))
		if err != nil {
			return nil, err
		}
		d.connectionCache.Set("all", conns)
		return conns, nil
	})
	if err != nil {
		return nil, err
	}
	return v.([]datacatalog.SourceConnection), nil
}

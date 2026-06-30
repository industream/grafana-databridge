package plugin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend/log"

	"github.com/industream/industream-data-bridge/pkg/cache"
	"github.com/industream/industream-data-bridge/pkg/datacatalog"
	"github.com/industream/industream-data-bridge/pkg/models"
)

// TestResolveStreamTargets_ResolvesByColumn simulates a cross-instance dashboard:
// GetEntriesByIds (foreign ids) returns nothing, but the all-entries listing
// yields the column match, so the stream still resolves a valid target.
func TestResolveStreamTargets_ResolvesByColumn(t *testing.T) {
	entry := dbEntry("local-id-B", "Mischung", fallbackTestColumn, "B040", "JB010")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// When fetching by ids (foreign ids), return an empty set.
		if r.URL.Query().Get("ids") != "" {
			_ = json.NewEncoder(w).Encode(datacatalog.PaginatedResponse[datacatalog.CatalogEntry]{
				Items: []datacatalog.CatalogEntry{}, TotalCount: 0,
			})
			return
		}
		// The all-entries listing yields the local column match.
		_ = json.NewEncoder(w).Encode(datacatalog.PaginatedResponse[datacatalog.CatalogEntry]{
			Items: []datacatalog.CatalogEntry{*entry}, TotalCount: 1,
		})
	}))
	defer srv.Close()

	ttl := time.Minute
	d := &Datasource{
		settings:        &models.PluginSettings{DataBridgeApiUrl: "http://databridge:8080"},
		logger:          log.DefaultLogger,
		catalogClient:   datacatalog.NewClient(srv.URL, ""),
		connectionCache: cache.NewStore[[]datacatalog.SourceConnection](ttl),
		entryCache:      cache.NewStore[[]datacatalog.CatalogEntry](ttl),
		labelCache:      cache.NewStore[[]datacatalog.Label](ttl),
		assetCache:      cache.NewStore[[]datacatalog.AssetDictionary](ttl),
		assetPathCache:  cache.NewStore[map[string]string](ttl),
	}

	selects := []models.SelectDefinition{
		{CatalogEntryId: "stale-guid-from-instance-A", Column: fallbackTestColumn, DataType: "double"},
	}

	targets, err := d.resolveStreamTargets(context.Background(), selects)
	if err != nil {
		t.Fatalf("resolveStreamTargets: %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("expected 1 stream target, got %d", len(targets))
	}
	if targets[0].databaseName != "B040" || targets[0].datasetName != "JB010" {
		t.Errorf("expected B040/JB010, got %s/%s", targets[0].databaseName, targets[0].datasetName)
	}
	if len(targets[0].selectItems) != 1 || targets[0].selectItems[0].Column != fallbackTestColumn {
		t.Errorf("expected SELECT column preserved, got %+v", targets[0].selectItems)
	}
}

package plugin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend/log"

	"github.com/industream/industream-data-bridge/pkg/cache"
	"github.com/industream/industream-data-bridge/pkg/datacatalog"
	"github.com/industream/industream-data-bridge/pkg/models"
)

// TestGetEntriesByIds_ServesFromCacheAndFetchesOnlyMissing proves the hot-path
// optimization: ids already present in the cached all-entries listing are served
// without hitting the API, and only the missing ids trigger a GetEntriesByIds call.
func TestGetEntriesByIds_ServesFromCacheAndFetchesOnlyMissing(t *testing.T) {
	var idsFetches int32
	var gotIds []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		ids := r.URL.Query()["ids"]
		if len(ids) > 0 {
			atomic.AddInt32(&idsFetches, 1)
			gotIds = ids
			_ = json.NewEncoder(w).Encode(datacatalog.PaginatedResponse[datacatalog.CatalogEntry]{
				Items: []datacatalog.CatalogEntry{{ID: "missing-2", Name: "Missing"}}, TotalCount: 1,
			})
			return
		}
		// all-entries listing (no ids) — should not be needed since cache is seeded.
		_ = json.NewEncoder(w).Encode(datacatalog.PaginatedResponse[datacatalog.CatalogEntry]{
			Items: []datacatalog.CatalogEntry{}, TotalCount: 0,
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

	// Seed the all-entries cache so cached ids never touch the API.
	d.entryCache.Set("all", []datacatalog.CatalogEntry{{ID: "cached-1", Name: "Cached"}})

	// All requested ids are cached → zero API fetches.
	got, err := d.getEntriesByIds(context.Background(), []string{"cached-1"})
	if err != nil {
		t.Fatalf("getEntriesByIds: %v", err)
	}
	if len(got) != 1 || got[0].ID != "cached-1" {
		t.Fatalf("expected cached-1 from cache, got %+v", got)
	}
	if n := atomic.LoadInt32(&idsFetches); n != 0 {
		t.Fatalf("expected 0 ids fetches for fully-cached ids, got %d", n)
	}

	// Mixed: one cached, one missing → exactly one fetch for the missing id only.
	got, err = d.getEntriesByIds(context.Background(), []string{"cached-1", "missing-2"})
	if err != nil {
		t.Fatalf("getEntriesByIds mixed: %v", err)
	}
	if n := atomic.LoadInt32(&idsFetches); n != 1 {
		t.Fatalf("expected 1 ids fetch for missing id, got %d", n)
	}
	if len(gotIds) != 1 || gotIds[0] != "missing-2" {
		t.Fatalf("expected only missing-2 to be fetched, got %v", gotIds)
	}
	byID := map[string]bool{}
	for i := range got {
		byID[got[i].ID] = true
	}
	if !byID["cached-1"] || !byID["missing-2"] {
		t.Fatalf("expected both cached-1 and missing-2 in result, got %+v", got)
	}
}

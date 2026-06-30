package plugin

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/data"

	"github.com/industream/industream-data-bridge/pkg/cache"
	"github.com/industream/industream-data-bridge/pkg/datacatalog"
	"github.com/industream/industream-data-bridge/pkg/models"
)

const fallbackTestColumn = "B040_JB010.MischungOrganischeranteil_0"

func ptrFloat(v float64) *float64 { return &v }

// dbEntry builds a DataBridge catalog entry with the given routing source params.
func dbEntry(id, name, column, database, dataset string) *datacatalog.CatalogEntry {
	return &datacatalog.CatalogEntry{
		ID:   id,
		Name: name,
		SourceConnection: &datacatalog.SourceConnection{
			ID:         "conn-databridge",
			Name:       "DataBridge",
			SourceType: &datacatalog.SourceType{ID: "DataBridge", Name: "DataBridge"},
		},
		DataType: "double",
		SourceParams: map[string]any{
			"column":   column,
			"database": database,
			"dataset":  dataset,
		},
	}
}

// newTestDatasource returns a Datasource with no catalog client (HTTP-free) so
// resolveConnectionUrl falls back to the configured DataBridgeApiUrl.
func newTestDatasource() *Datasource {
	ttl := time.Minute
	return &Datasource{
		settings: &models.PluginSettings{
			DataBridgeApiUrl:   "http://databridge:8080",
			DefaultDisplayName: "entryName",
		},
		logger:          log.DefaultLogger,
		connectionCache: cache.NewStore[[]datacatalog.SourceConnection](ttl),
		entryCache:      cache.NewStore[[]datacatalog.CatalogEntry](ttl),
		labelCache:      cache.NewStore[[]datacatalog.Label](ttl),
		assetCache:      cache.NewStore[[]datacatalog.AssetDictionary](ttl),
		assetPathCache:  cache.NewStore[map[string]string](ttl),
	}
}

// TestGroupByTarget_UnknownEntryIdResolvesByColumn is the acceptance criterion:
// a select whose foreign catalogEntryId is unknown on this instance resolves to
// the local DataBridge target via the stable column key, producing the exact
// same target/query as the same-instance id path.
func TestGroupByTarget_UnknownEntryIdResolvesByColumn(t *testing.T) {
	d := newTestDatasource()
	entry := dbEntry("local-id-B", "Mischung", fallbackTestColumn, "B040", "JB010")

	entryMap := map[string]*datacatalog.CatalogEntry{} // foreign id absent on instance B
	byColumn := map[string]*datacatalog.CatalogEntry{fallbackTestColumn: entry}

	selects := []models.SelectDefinition{
		{CatalogEntryId: "stale-guid-from-instance-A", Column: fallbackTestColumn, DataType: "double"},
	}

	targets, err := d.groupByTarget(context.Background(), selects, entryMap, byColumn)
	if err != nil {
		t.Fatalf("groupByTarget: %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}
	tg := targets[0]
	if tg.databaseName != "B040" || tg.datasetName != "JB010" {
		t.Errorf("expected B040/JB010, got %s/%s", tg.databaseName, tg.datasetName)
	}
	if tg.bridgeUrl != "http://databridge:8080" {
		t.Errorf("expected fallback bridgeUrl, got %s", tg.bridgeUrl)
	}
	if len(tg.selectItems) != 1 || tg.selectItems[0].Column != fallbackTestColumn {
		t.Fatalf("expected select column preserved, got %+v", tg.selectItems)
	}

	// The emitted RecordsQuery must be byte-identical to the same-instance id path.
	now := time.Now()
	tr := backend.TimeRange{From: now.Add(-time.Hour), To: now}

	idSelects := []models.SelectDefinition{
		{CatalogEntryId: "local-id-B", Column: fallbackTestColumn, DataType: "double"},
	}
	idEntryMap := map[string]*datacatalog.CatalogEntry{"local-id-B": entry}
	idTargets, err := d.groupByTarget(context.Background(), idSelects, idEntryMap, nil)
	if err != nil {
		t.Fatalf("groupByTarget id path: %v", err)
	}

	colQd := &models.QueryDefinition{Mode: "dataCatalog", Select: tg.selectItems}
	idQd := &models.QueryDefinition{Mode: "dataCatalog", Select: idTargets[0].selectItems}
	colRq := buildRecordsQuery(colQd, tr, 1000)
	idRq := buildRecordsQuery(idQd, tr, 1000)
	if !reflect.DeepEqual(colRq, idRq) {
		t.Errorf("column-resolved RecordsQuery differs from id-resolved:\ncol=%+v\nid =%+v", colRq, idRq)
	}
}

// TestGroupByTarget_KnownEntryId_IdWins proves id resolution still wins when the
// same column also exists in byColumn pointing at a different target.
func TestGroupByTarget_KnownEntryId_IdWins(t *testing.T) {
	d := newTestDatasource()
	idEntry := dbEntry("local-id-B", "Mischung", fallbackTestColumn, "B040", "JB010")
	colEntry := dbEntry("other-id", "Other", fallbackTestColumn, "OTHERDB", "OTHERDS")

	entryMap := map[string]*datacatalog.CatalogEntry{"local-id-B": idEntry}
	byColumn := map[string]*datacatalog.CatalogEntry{fallbackTestColumn: colEntry}

	selects := []models.SelectDefinition{
		{CatalogEntryId: "local-id-B", Column: fallbackTestColumn, DataType: "double"},
	}

	targets, err := d.groupByTarget(context.Background(), selects, entryMap, byColumn)
	if err != nil {
		t.Fatalf("groupByTarget: %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}
	if targets[0].databaseName != "B040" || targets[0].datasetName != "JB010" {
		t.Errorf("id must win: expected B040/JB010, got %s/%s", targets[0].databaseName, targets[0].datasetName)
	}
}

func TestGroupByTarget_BothMiss_Skips(t *testing.T) {
	d := newTestDatasource()
	entryMap := map[string]*datacatalog.CatalogEntry{}
	byColumn := map[string]*datacatalog.CatalogEntry{}

	selects := []models.SelectDefinition{
		{CatalogEntryId: "unknown", Column: "no-such-column", DataType: "double"},
	}

	targets, err := d.groupByTarget(context.Background(), selects, entryMap, byColumn)
	if err != nil {
		t.Fatalf("groupByTarget: %v", err)
	}
	if len(targets) != 0 {
		t.Fatalf("expected 0 targets (skip), got %d", len(targets))
	}
}

func TestResolveEntry(t *testing.T) {
	idEntry := dbEntry("id-1", "A", "colA", "DB", "DS")
	colEntry := dbEntry("id-2", "B", "colB", "DB", "DS")

	entryMap := map[string]*datacatalog.CatalogEntry{"id-1": idEntry}
	byColumn := map[string]*datacatalog.CatalogEntry{"colB": colEntry}

	tests := []struct {
		name string
		sel  models.SelectDefinition
		want *datacatalog.CatalogEntry
	}{
		{"id hit", models.SelectDefinition{CatalogEntryId: "id-1", Column: "colA"}, idEntry},
		{"id miss column hit", models.SelectDefinition{CatalogEntryId: "stale", Column: "colB"}, colEntry},
		{"empty id column hit", models.SelectDefinition{Column: "colB"}, colEntry},
		{"both miss", models.SelectDefinition{CatalogEntryId: "stale", Column: "colX"}, nil},
		{"empty column id miss", models.SelectDefinition{CatalogEntryId: "stale"}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveEntry(&tt.sel, entryMap, byColumn)
			if got != tt.want {
				t.Errorf("resolveEntry = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDataBridgeEntriesByColumn_CollisionKeepsFirst(t *testing.T) {
	d := newTestDatasource()
	first := dbEntry("id-1", "First", "sharedCol", "DB1", "DS1")
	second := dbEntry("id-2", "Second", "sharedCol", "DB2", "DS2")
	unique := dbEntry("id-3", "Third", "uniqueCol", "DB3", "DS3")

	// Seed the entry cache so getAllEntries serves without a catalog client.
	d.entryCache.Set("all", []datacatalog.CatalogEntry{*first, *second, *unique})

	idx := d.dataBridgeEntriesByColumn(context.Background())
	if got := idx["sharedCol"]; got == nil || got.ID != "id-1" {
		t.Errorf("collision must keep first (id-1), got %+v", got)
	}
	if got := idx["uniqueCol"]; got == nil || got.ID != "id-3" {
		t.Errorf("expected uniqueCol -> id-3, got %+v", got)
	}
}

func TestApplyDisplayNames_ColumnResolvedKeepsMetadata(t *testing.T) {
	d := newTestDatasource()
	entry := dbEntry("local-id-B", "Mischung", fallbackTestColumn, "B040", "JB010")
	entry.Metadata = &datacatalog.CatalogMetadata{
		Unit: datacatalog.FlexString("%"),
		Min:  datacatalog.FlexFloat64{Value: ptrFloat(0)},
		Max:  datacatalog.FlexFloat64{Value: ptrFloat(100)},
	}

	entryMap := map[string]*datacatalog.CatalogEntry{}
	byColumn := map[string]*datacatalog.CatalogEntry{fallbackTestColumn: entry}

	qd := &models.QueryDefinition{
		Select: []models.SelectDefinition{
			{CatalogEntryId: "stale-guid", Column: fallbackTestColumn, DataType: "double"},
		},
	}

	frame := data.NewFrame("test",
		data.NewField(fallbackTestColumn, nil, []float64{1, 2, 3}),
	)

	d.applyDisplayNamesFromMap(frame, qd, entryMap, byColumn)

	fc := frame.Fields[0].Config
	if fc == nil {
		t.Fatal("expected field config to be set via column fallback")
	}
	if fc.Unit != "%" {
		t.Errorf("expected unit '%%', got %q", fc.Unit)
	}
	if fc.Thresholds == nil {
		t.Error("expected thresholds from min/max via column fallback")
	}
}

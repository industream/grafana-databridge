package plugin

import (
	"context"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

func TestNewDatasource_WithValidSettings(t *testing.T) {
	settings := backend.DataSourceInstanceSettings{
		JSONData: []byte(`{
			"dataBridgeApiUrl": "http://localhost:8002",
			"dataCatalogApiUrl": "http://localhost:8010",
			"maxRawRows": 50000,
			"hardLimitRows": 1000000,
			"cacheTtlSeconds": 300
		}`),
		DecryptedSecureJSONData: map[string]string{
			"apiKey": "test-key",
		},
	}

	instance, err := NewDatasource(context.Background(), settings)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ds, ok := instance.(*Datasource)
	if !ok {
		t.Fatal("expected *Datasource")
	}

	if ds.settings.DataBridgeApiUrl != "http://localhost:8002" {
		t.Errorf("expected DataBridgeApiUrl = http://localhost:8002, got %s", ds.settings.DataBridgeApiUrl)
	}

	if ds.settings.DataCatalogApiUrl != "http://localhost:8010" {
		t.Errorf("expected DataCatalogApiUrl = http://localhost:8010, got %s", ds.settings.DataCatalogApiUrl)
	}

	if ds.catalogClient == nil {
		t.Error("expected catalogClient to be initialized")
	}
}

func TestNewDatasource_WithDefaults(t *testing.T) {
	settings := backend.DataSourceInstanceSettings{
		JSONData:                []byte(`{}`),
		DecryptedSecureJSONData: map[string]string{},
	}

	instance, err := NewDatasource(context.Background(), settings)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ds := instance.(*Datasource)

	if ds.settings.MaxRawRows != 50_000 {
		t.Errorf("expected MaxRawRows default 50000, got %d", ds.settings.MaxRawRows)
	}
	if ds.settings.HardLimitRows != 1_000_000 {
		t.Errorf("expected HardLimitRows default 1000000, got %d", ds.settings.HardLimitRows)
	}
	if ds.settings.CacheTtlSeconds != 300 {
		t.Errorf("expected CacheTtlSeconds default 300, got %d", ds.settings.CacheTtlSeconds)
	}
	if ds.settings.DefaultAggregation != "avg" {
		t.Errorf("expected DefaultAggregation default avg, got %s", ds.settings.DefaultAggregation)
	}
	if ds.catalogClient != nil {
		t.Error("expected catalogClient to be nil when no URL configured")
	}
}

func TestNewDatasource_WithInvalidJSON(t *testing.T) {
	settings := backend.DataSourceInstanceSettings{
		JSONData: []byte(`invalid`),
	}

	_, err := NewDatasource(context.Background(), settings)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

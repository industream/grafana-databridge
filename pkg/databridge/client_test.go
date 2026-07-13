package databridge

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ComputeStats must scope the request to a dataset (measurement) so DataBridge does not
// pool a field name shared across measurements. The datasetName is sent as a query param.
func TestComputeStats_SendsDatasetNameQueryParam(t *testing.T) {
	var gotDB, gotDataset string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotDB = r.URL.Query().Get("databaseName")
		gotDataset = r.URL.Query().Get("datasetName")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Temperature":{"mean":1210.5}}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	resp, err := client.ComputeStats(context.Background(), "IronStream", "HotBlast",
		&StatsQuery{Entries: []string{"Temperature"}, Start: "2026-07-13T00:00:00Z"})
	if err != nil {
		t.Fatalf("ComputeStats returned error: %v", err)
	}

	if gotDB != "IronStream" {
		t.Errorf("databaseName = %q, want IronStream", gotDB)
	}
	if gotDataset != "HotBlast" {
		t.Errorf("datasetName = %q, want HotBlast (must be scoped to the measurement)", gotDataset)
	}
	if resp["Temperature"]["mean"] != 1210.5 {
		t.Errorf("mean = %v, want 1210.5", resp["Temperature"]["mean"])
	}
}

// An empty datasetName preserves the legacy whole-database behavior: the param is omitted.
func TestComputeStats_OmitsDatasetNameWhenEmpty(t *testing.T) {
	hasDataset := true
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, hasDataset = r.URL.Query()["datasetName"]
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	if _, err := client.ComputeStats(context.Background(), "IronStream", "",
		&StatsQuery{Entries: []string{"ramp"}, Start: "2026-07-13T00:00:00Z"}); err != nil {
		t.Fatalf("ComputeStats returned error: %v", err)
	}

	if hasDataset {
		t.Error("datasetName query param should be absent when datasetName is empty")
	}
}

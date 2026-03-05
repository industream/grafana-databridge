package plugin

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/resource/httpadapter"
)

// CallResource handles resource calls from the frontend (dropdowns, catalog data).
func (d *Datasource) CallResource(ctx context.Context, req *backend.CallResourceRequest, sender backend.CallResourceResponseSender) error {
	return httpadapter.New(d.resourceHandler()).CallResource(ctx, req, sender)
}

func (d *Datasource) resourceHandler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /connections", d.handleGetConnections)
	mux.HandleFunc("GET /databases", d.handleGetDatabases)
	mux.HandleFunc("GET /datasets", d.handleGetDatasets)
	mux.HandleFunc("GET /schema", d.handleGetSchema)
	mux.HandleFunc("GET /catalog-entries", d.handleGetCatalogEntries)
	mux.HandleFunc("GET /asset-tree", d.handleGetAssetTree)
	mux.HandleFunc("GET /labels", d.handleGetLabels)

	return mux
}

func (d *Datasource) handleGetConnections(w http.ResponseWriter, r *http.Request) {
	conns, err := d.getConnections(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, conns)
}

func (d *Datasource) handleGetDatabases(w http.ResponseWriter, r *http.Request) {
	connectionId := r.URL.Query().Get("connectionId")
	bridgeUrl, err := d.resolveConnectionUrl(r.Context(), connectionId)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	client := d.dataBridgeClient(bridgeUrl)
	databases, err := client.ListDatabases(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, databases)
}

func (d *Datasource) handleGetDatasets(w http.ResponseWriter, r *http.Request) {
	connectionId := r.URL.Query().Get("connectionId")
	databaseName := r.URL.Query().Get("database")

	bridgeUrl, err := d.resolveConnectionUrl(r.Context(), connectionId)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	client := d.dataBridgeClient(bridgeUrl)
	datasets, err := client.ListDatasets(r.Context(), databaseName)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, datasets)
}

func (d *Datasource) handleGetSchema(w http.ResponseWriter, r *http.Request) {
	connectionId := r.URL.Query().Get("connectionId")
	databaseName := r.URL.Query().Get("database")
	datasetName := r.URL.Query().Get("dataset")

	bridgeUrl, err := d.resolveConnectionUrl(r.Context(), connectionId)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	client := d.dataBridgeClient(bridgeUrl)
	schema, err := client.GetSchema(r.Context(), databaseName, datasetName)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, schema)
}

func (d *Datasource) handleGetCatalogEntries(w http.ResponseWriter, r *http.Request) {
	if d.catalogClient == nil {
		writeError(w, http.StatusServiceUnavailable, "DataCatalog is not configured")
		return
	}

	ids := r.URL.Query().Get("ids")
	label := r.URL.Query().Get("label")
	search := r.URL.Query().Get("search")

	var entries interface{}
	var err error

	if ids != "" {
		entries, err = d.catalogClient.GetEntriesByIds(r.Context(), splitIds(ids))
	} else {
		entries, err = d.catalogClient.ListEntries(r.Context(), label, search)
	}

	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, entries)
}

func (d *Datasource) handleGetAssetTree(w http.ResponseWriter, r *http.Request) {
	if d.catalogClient == nil {
		writeError(w, http.StatusServiceUnavailable, "DataCatalog is not configured")
		return
	}

	if trees, ok := d.assetCache.Get("all"); ok {
		writeJSON(w, trees)
		return
	}

	trees, err := d.catalogClient.ListAssetDictionaries(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	d.assetCache.Set("all", trees)
	writeJSON(w, trees)
}

func (d *Datasource) handleGetLabels(w http.ResponseWriter, r *http.Request) {
	if d.catalogClient == nil {
		writeError(w, http.StatusServiceUnavailable, "DataCatalog is not configured")
		return
	}

	if labels, ok := d.labelCache.Get("all"); ok {
		writeJSON(w, labels)
		return
	}

	labels, err := d.catalogClient.ListLabels(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	d.labelCache.Set("all", labels)
	writeJSON(w, labels)
}

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func splitIds(ids string) []string {
	if ids == "" {
		return nil
	}
	var result []string
	for _, id := range strings.Split(ids, ",") {
		trimmed := strings.TrimSpace(id)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

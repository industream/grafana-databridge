package plugin

import (
	"encoding/json"
	"net/http"
)

// variableOption represents a single option in a Grafana variable dropdown.
type variableOption struct {
	Text  string `json:"text"`
	Value string `json:"value"`
}

// handleGetVariables handles variable query requests from Grafana dashboard variables.
// Query param "type" determines which list to return:
//   - connections: list source connections
//   - databases:   list databases for a connection (requires connectionId)
//   - datasets:    list datasets for a connection+database (requires connectionId, database)
//   - entries:     list catalog entries (optional: label, search)
//   - labels:      list labels
func (d *Datasource) handleGetVariables(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	queryType := r.URL.Query().Get("type")

	switch queryType {
	case "connections":
		conns, err := d.getConnections(ctx)
		if err != nil {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
		options := make([]variableOption, len(conns))
		for i, c := range conns {
			options[i] = variableOption{Text: c.Name, Value: c.ID}
		}
		writeJSON(w, options)

	case "databases":
		connectionId := r.URL.Query().Get("connectionId")
		bridgeUrl, err := d.resolveConnectionUrl(ctx, connectionId)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		client := d.dataBridgeClient(bridgeUrl)
		databases, err := client.ListDatabases(ctx)
		if err != nil {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
		options := make([]variableOption, len(databases))
		for i, db := range databases {
			options[i] = variableOption{Text: db.Name, Value: db.Name}
		}
		writeJSON(w, options)

	case "datasets":
		connectionId := r.URL.Query().Get("connectionId")
		databaseName := r.URL.Query().Get("database")
		bridgeUrl, err := d.resolveConnectionUrl(ctx, connectionId)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		client := d.dataBridgeClient(bridgeUrl)
		datasets, err := client.ListDatasets(ctx, databaseName)
		if err != nil {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
		options := make([]variableOption, len(datasets))
		for i, ds := range datasets {
			options[i] = variableOption{Text: ds.Name, Value: ds.Name}
		}
		writeJSON(w, options)

	case "entries":
		if d.catalogClient == nil {
			writeJSON(w, []variableOption{})
			return
		}
		label := r.URL.Query().Get("label")
		search := r.URL.Query().Get("search")
		entries, err := d.catalogClient.ListEntries(ctx, label, search)
		if err != nil {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
		options := make([]variableOption, len(entries))
		for i, e := range entries {
			options[i] = variableOption{Text: e.Name, Value: e.ID}
		}
		writeJSON(w, options)

	case "labels":
		if d.catalogClient == nil {
			writeJSON(w, []variableOption{})
			return
		}
		if labels, ok := d.labelCache.Get("all"); ok {
			options := make([]variableOption, len(labels))
			for i, l := range labels {
				options[i] = variableOption{Text: l.Name, Value: l.ID}
			}
			writeJSON(w, options)
			return
		}
		labels, err := d.catalogClient.ListLabels(ctx)
		if err != nil {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
		d.labelCache.Set("all", labels)
		options := make([]variableOption, len(labels))
		for i, l := range labels {
			options[i] = variableOption{Text: l.Name, Value: l.ID}
		}
		writeJSON(w, options)

	default:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "unknown variable type: " + queryType,
			"hint":  "valid types: connections, databases, datasets, entries, labels",
		})
	}
}

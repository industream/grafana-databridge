package plugin

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/resource/httpadapter"

	"github.com/industream/industream-data-bridge/pkg/datacatalog"
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
	mux.HandleFunc("GET /node-entries", d.handleGetNodeEntries)
	mux.HandleFunc("GET /labels", d.handleGetLabels)
	mux.HandleFunc("GET /variables", d.handleGetVariables)
	mux.HandleFunc("POST /cache/clear", d.handleClearCache)

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

	var entries []datacatalog.CatalogEntry
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
	writeJSON(w, d.filterEntriesByConnection(entries))
}

func (d *Datasource) handleGetAssetTree(w http.ResponseWriter, r *http.Request) {
	if d.catalogClient == nil {
		writeError(w, http.StatusServiceUnavailable, "DataCatalog is not configured")
		return
	}

	d.ensureAssetTreeCached(r.Context())

	trees, ok := d.assetCache.Get("all")
	if !ok {
		writeError(w, http.StatusBadGateway, "failed to load asset tree")
		return
	}
	writeJSON(w, trees)
}

func (d *Datasource) handleGetNodeEntries(w http.ResponseWriter, r *http.Request) {
	if d.catalogClient == nil {
		writeError(w, http.StatusServiceUnavailable, "DataCatalog is not configured")
		return
	}

	nodeId := r.URL.Query().Get("nodeId")
	if nodeId == "" {
		writeError(w, http.StatusBadRequest, "nodeId is required")
		return
	}

	// Ensure the asset tree is cached so we can look up entryIds.
	d.ensureAssetTreeCached(r.Context())

	ids := d.findNodeEntryIds(nodeId)
	if len(ids) == 0 {
		writeJSON(w, []interface{}{})
		return
	}

	entries, err := d.catalogClient.GetEntriesByIds(r.Context(), ids)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, d.filterEntriesByConnection(entries))
}

// ensureAssetTreeCached loads and caches the asset tree if not already cached.
// When sourceConnectionId is configured, entry IDs are filtered to only include
// entries from the matching connection, and entryCount is updated accordingly.
func (d *Datasource) ensureAssetTreeCached(ctx context.Context) {
	if _, ok := d.assetCache.Get("all"); ok {
		return
	}
	trees, err := d.catalogClient.ListAssetDictionaries(ctx)
	if err != nil {
		d.logger.Warn("Failed to load asset dictionaries for cache", "error", err)
		return
	}

	// Build a mapping from non-DataBridge entry IDs to their DataBridge equivalents (same name).
	// This allows asset trees that reference MQTT/OPC-UA entries to resolve to queryable DataBridge entries.
	allEntries, err := d.catalogClient.ListEntries(ctx, "", "")
	var entryIdRemap map[string]string // non-DB entry ID → DataBridge entry ID
	var validEntryIds map[string]bool
	if err == nil {
		// Index DataBridge entries by name
		dbByName := make(map[string]string)
		for _, e := range allEntries {
			if e.IsDataBridgeEntry() {
				if d.settings.SourceConnectionId != "" && e.GetSourceConnectionID() != d.settings.SourceConnectionId {
					continue
				}
				dbByName[e.Name] = e.ID
			}
		}
		// Build remap: for non-DataBridge entries, find matching DataBridge entry by name
		entryIdRemap = make(map[string]string)
		validEntryIds = make(map[string]bool)
		for _, e := range allEntries {
			if e.IsDataBridgeEntry() {
				validEntryIds[e.ID] = true
			} else if dbId, ok := dbByName[e.Name]; ok {
				entryIdRemap[e.ID] = dbId
				validEntryIds[dbId] = true
			}
		}
	}

	for i := range trees {
		flatNodes, err := d.catalogClient.ListAssetNodes(ctx, trees[i].ID)
		if err != nil {
			d.logger.Warn("Failed to fetch nodes for dictionary", "id", trees[i].ID, "error", err)
			continue
		}
		trees[i].Nodes = buildNodeTree(flatNodes)
		if entryIdRemap != nil {
			remapTreeEntryIds(trees[i].Nodes, entryIdRemap, validEntryIds)
		}
	}
	d.assetCache.Set("all", trees)
}

// remapTreeEntryIds replaces non-DataBridge entry IDs with their DataBridge equivalents
// and removes entries that have no DataBridge counterpart.
func remapTreeEntryIds(nodes []datacatalog.AssetNode, remap map[string]string, validIds map[string]bool) {
	for i := range nodes {
		seen := make(map[string]bool)
		remapped := make([]string, 0, len(nodes[i].EntryIds))
		for _, id := range nodes[i].EntryIds {
			// Remap non-DataBridge IDs to their DataBridge counterpart
			if newId, ok := remap[id]; ok {
				id = newId
			}
			// Only keep valid (DataBridge) entries, deduplicate
			if validIds[id] && !seen[id] {
				remapped = append(remapped, id)
				seen[id] = true
			}
		}
		nodes[i].EntryIds = remapped
		nodes[i].EntryCount = len(remapped)
		if len(nodes[i].Children) > 0 {
			remapTreeEntryIds(nodes[i].Children, remap, validIds)
		}
	}
}

// buildNodeTree converts a flat list of nodes with parentId into a nested tree.
func buildNodeTree(flat []datacatalog.AssetNode) []datacatalog.AssetNode {
	byID := make(map[string]*datacatalog.AssetNode, len(flat))
	for i := range flat {
		flat[i].Children = nil // reset
		byID[flat[i].ID] = &flat[i]
	}

	var roots []datacatalog.AssetNode
	for i := range flat {
		n := &flat[i]
		if n.ParentID == nil {
			roots = append(roots, *n)
		} else if parent, ok := byID[*n.ParentID]; ok {
			parent.Children = append(parent.Children, *n)
		}
	}

	// Copy nested children back from byID map to roots
	var buildTree func(id string) datacatalog.AssetNode
	buildTree = func(id string) datacatalog.AssetNode {
		n := byID[id]
		result := *n
		result.EntryCount = len(result.EntryIds)
		result.Children = make([]datacatalog.AssetNode, 0)
		for _, child := range n.Children {
			nested := buildTree(child.ID)
			result.Children = append(result.Children, nested)
		}
		return result
	}

	result := make([]datacatalog.AssetNode, 0, len(roots))
	for _, root := range roots {
		result = append(result, buildTree(root.ID))
	}
	return result
}

// getAssetPaths returns a cached map of entryId -> asset path string (e.g. "Plant > PostgreSQL > Counter").
func (d *Datasource) getAssetPaths() map[string]string {
	if paths, ok := d.assetPathCache.Get("all"); ok {
		return paths
	}

	// Ensure asset tree is loaded
	if d.catalogClient != nil {
		d.ensureAssetTreeCached(context.Background())
	}

	trees, ok := d.assetCache.Get("all")
	if !ok {
		return nil
	}

	paths := make(map[string]string)
	for _, tree := range trees {
		buildAssetPaths(tree.Nodes, nil, paths)
	}
	d.assetPathCache.Set("all", paths)
	return paths
}

// buildAssetPaths recursively builds entry paths from the asset tree.
func buildAssetPaths(nodes []datacatalog.AssetNode, ancestors []string, paths map[string]string) {
	for _, node := range nodes {
		currentPath := append(append([]string{}, ancestors...), node.Name)
		pathStr := strings.Join(currentPath, " > ")
		for _, entryId := range node.EntryIds {
			paths[entryId] = pathStr
		}
		if len(node.Children) > 0 {
			buildAssetPaths(node.Children, currentPath, paths)
		}
	}
}

// findNodeEntryIds searches cached asset trees for a node's entryIds.
func (d *Datasource) findNodeEntryIds(nodeId string) []string {
	trees, ok := d.assetCache.Get("all")
	if !ok {
		return nil
	}
	for _, tree := range trees {
		if ids := findInNodes(tree.Nodes, nodeId); ids != nil {
			return ids
		}
	}
	return nil
}

func findInNodes(nodes []datacatalog.AssetNode, nodeId string) []string {
	for _, n := range nodes {
		if n.ID == nodeId {
			return n.EntryIds
		}
		if ids := findInNodes(n.Children, nodeId); ids != nil {
			return ids
		}
	}
	return nil
}

// filterEntriesByConnection filters entries to only include queryable DataBridge entries.
// Non-DataBridge entries (MQTT, OPC-UA, etc.) are always excluded since they cannot be queried.
// If sourceConnectionId is configured, entries are further filtered to that specific connection.
func (d *Datasource) filterEntriesByConnection(entries []datacatalog.CatalogEntry) []datacatalog.CatalogEntry {
	connId := d.settings.SourceConnectionId
	filtered := make([]datacatalog.CatalogEntry, 0, len(entries))
	for _, e := range entries {
		// Only include entries with a DataBridge source connection
		if !e.IsDataBridgeEntry() {
			continue
		}
		// If a specific connection is configured, further filter
		if connId != "" && e.GetSourceConnectionID() != connId {
			continue
		}
		filtered = append(filtered, e)
	}
	return filtered
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

func (d *Datasource) handleClearCache(w http.ResponseWriter, r *http.Request) {
	d.connectionCache.Clear()
	d.entryCache.Clear()
	d.labelCache.Clear()
	d.assetCache.Clear()
	d.assetPathCache.Clear()
	d.logger.Info("Cache cleared by user")
	writeJSON(w, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
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

package plugin

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"

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

	// Node entryIds are logical ids. GetEntriesByIds matches on the binding id
	// (?ids=), so it would miss them. Resolve against the DataBridge entries by
	// logical id instead. getAllEntries serves the cached, DataBridge-scoped list.
	allEntries, err := d.getAllEntries(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	entries := selectEntriesByLogicalIds(allEntries, ids)
	writeJSON(w, d.filterEntriesByConnection(entries))
}

// ensureAssetTreeCached loads and caches the asset tree if not already cached.
// When sourceConnectionId is configured, entry IDs are filtered to only include
// entries from the matching connection, and entryCount is updated accordingly.
func (d *Datasource) ensureAssetTreeCached(ctx context.Context) {
	if _, ok := d.assetCache.Get("all"); ok {
		return
	}
	// Collapse concurrent cold-cache misses into one build (thundering-herd guard).
	_, _, _ = d.sf.Do("assetTree", func() (interface{}, error) {
		if _, ok := d.assetCache.Get("all"); ok {
			return nil, nil
		}
		// Detach cancellation: this build is shared by every singleflight joiner,
		// so the first caller's cancellation must not abort it for the others.
		d.loadAssetTree(context.WithoutCancel(ctx))
		return nil, nil
	})
}

// loadAssetTree fetches the asset dictionaries and their nodes, filters node
// entryIds to the valid DataBridge entry set, and populates assetCache.
func (d *Datasource) loadAssetTree(ctx context.Context) {
	trees, err := d.catalogClient.ListAssetDictionaries(ctx)
	if err != nil {
		d.logger.Warn("Failed to load asset dictionaries for cache", "error", err)
		// Cache empty list to prevent repeated failed requests
		d.assetCache.Set("all", []datacatalog.AssetDictionary{})
		return
	}

	// Parent/binding model: a DataBridge entry's own id (CatalogEntry.ID) differs
	// from its logical id (CatalogEntry.EntryID). Asset nodes reference the logical
	// id, so the valid set must be keyed on GetLogicalID() — keying on .ID drops
	// every node entry. getAllEntries serves the cached, DataBridge-scoped list.
	allEntries, err := d.getAllEntries(ctx)
	var validEntryIds map[string]bool
	if err == nil {
		validEntryIds = buildValidEntryIds(allEntries, d.settings.SourceConnectionId)
	}

	// Fetch each dictionary's nodes in parallel with bounded concurrency. Each
	// goroutine writes a distinct trees[i], so no synchronization is needed.
	const maxConcurrency = 8
	sem := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup
	for i := range trees {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()

			flatNodes, err := d.catalogClient.ListAssetNodes(ctx, trees[i].ID)
			if err != nil {
				d.logger.Warn("Failed to fetch nodes for dictionary", "id", trees[i].ID, "error", err)
				return
			}
			trees[i].Nodes = buildNodeTree(flatNodes)
			if validEntryIds != nil {
				filterTreeEntryIds(trees[i].Nodes, validEntryIds)
			}
		}(i)
	}
	wg.Wait()
	d.assetCache.Set("all", trees)
}

// remapTreeEntryIds replaces non-DataBridge entry IDs with their DataBridge equivalents
// and removes entries that have no DataBridge counterpart.
// filterTreeEntryIds keeps only entry IDs that are valid DataBridge entries.
func filterTreeEntryIds(nodes []datacatalog.AssetNode, validIds map[string]bool) {
	for i := range nodes {
		filtered := make([]string, 0, len(nodes[i].EntryIds))
		for _, id := range nodes[i].EntryIds {
			if validIds[id] {
				filtered = append(filtered, id)
			}
		}
		nodes[i].EntryIds = filtered
		nodes[i].EntryCount = len(filtered)
		if len(nodes[i].Children) > 0 {
			filterTreeEntryIds(nodes[i].Children, validIds)
		}
	}
}

// buildValidEntryIds returns the set of logical entry IDs present in entries
// (keyed by GetLogicalID), optionally restricted to a single source connection.
// Asset dictionary nodes reference logical IDs, so this is the correct match key
// for filterTreeEntryIds.
func buildValidEntryIds(entries []datacatalog.CatalogEntry, connId string) map[string]bool {
	valid := make(map[string]bool, len(entries))
	for i := range entries {
		e := &entries[i]
		if connId != "" && e.GetSourceConnectionID() != connId {
			continue
		}
		valid[e.GetLogicalID()] = true
	}
	return valid
}

// selectEntriesByLogicalIds returns the entries whose logical id (GetLogicalID)
// is in logicalIds. Used to resolve asset-node entryIds (logical) to the actual
// DataBridge binding entries, since GetEntriesByIds matches on the binding id.
func selectEntriesByLogicalIds(entries []datacatalog.CatalogEntry, logicalIds []string) []datacatalog.CatalogEntry {
	want := make(map[string]bool, len(logicalIds))
	for _, id := range logicalIds {
		want[id] = true
	}
	out := make([]datacatalog.CatalogEntry, 0, len(logicalIds))
	for i := range entries {
		if want[entries[i].GetLogicalID()] {
			out = append(out, entries[i])
		}
	}
	return out
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
// It runs on the query hot path, so the caller's ctx is threaded through to the
// asset-tree load so query deadline/cancellation propagate.
func (d *Datasource) getAssetPaths(ctx context.Context) map[string]string {
	if paths, ok := d.assetPathCache.Get("all"); ok {
		return paths
	}

	// Ensure asset tree is loaded
	if d.catalogClient != nil {
		d.ensureAssetTreeCached(ctx)
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

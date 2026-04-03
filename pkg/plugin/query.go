package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"

	"github.com/industream/industream-data-bridge/pkg/datacatalog"
	"github.com/industream/industream-data-bridge/pkg/databridge"
	"github.com/industream/industream-data-bridge/pkg/displayname"
	"github.com/industream/industream-data-bridge/pkg/models"
)

// QueryData handles multiple queries and returns multiple responses.
func (d *Datasource) QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	response := backend.NewQueryDataResponse()

	for _, q := range req.Queries {
		response.Responses[q.RefID] = d.handleQuery(ctx, q)
	}

	return response, nil
}

func (d *Datasource) handleQuery(ctx context.Context, query backend.DataQuery) backend.DataResponse {
	var qd models.QueryDefinition
	if err := json.Unmarshal(query.JSON, &qd); err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("unmarshal query: %v", err))
	}
	qd.ParseWhere()

	mode := qd.Mode
	if mode == "" {
		mode = "dataCatalog"
	}

	switch mode {
	case "raw":
		return d.handleRawQuery(ctx, query, &qd)
	case "dataCatalog":
		return d.handleCatalogQuery(ctx, query, &qd)
	default:
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("unknown mode: %s", mode))
	}
}

// resolveAggregations resolves "optimized" and incompatible aggregations based on DataType.
func resolveAggregations(selectItems []models.SelectDefinition) {
	for i := range selectItems {
		s := &selectItems[i]
		if s.Aggregation == "" || s.Aggregation == "optimized" {
			s.Aggregation = compatibleAggregation(s.DataType)
		} else if !isAggregationCompatible(s.Aggregation, s.DataType) {
			s.Aggregation = compatibleAggregation(s.DataType)
		}
	}
}

// handleRawQuery executes a query in raw mode (single connection from config).
func (d *Datasource) handleRawQuery(ctx context.Context, query backend.DataQuery, qd *models.QueryDefinition) backend.DataResponse {
	bridgeUrl, err := d.resolveConnectionUrl(ctx, qd.ConnectionId)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusInternal, fmt.Sprintf("resolve connection: %v", err))
	}

	if qd.DatabaseName == "" || qd.DatasetName == "" {
		return backend.ErrDataResponse(backend.StatusBadRequest, "database and dataset are required")
	}

	resolveAggregations(qd.Select)
	d.applySafetyLimits(qd, query.TimeRange)

	recordsQuery := buildRecordsQuery(qd, query.TimeRange, query.MaxDataPoints)
	return d.executeAndConvert(ctx, bridgeUrl, qd.DatabaseName, qd.DatasetName, query.RefID, recordsQuery)
}

// queryTarget groups select items that share the same DataBridge connection, database, and dataset.
type queryTarget struct {
	bridgeUrl    string
	databaseName string
	datasetName  string
	selectItems  []models.SelectDefinition
}

// handleCatalogQuery executes a query in dataCatalog mode.
// Tags are grouped by their source connection + database + dataset, and each group
// is queried in parallel against its respective DataBridge instance.
func (d *Datasource) handleCatalogQuery(ctx context.Context, query backend.DataQuery, qd *models.QueryDefinition) backend.DataResponse {
	if d.catalogClient == nil {
		return backend.ErrDataResponse(backend.StatusInternal, "DataCatalog URL is not configured")
	}

	// Collect all catalog entry IDs from select definitions
	ids := make([]string, 0, len(qd.Select))
	for _, s := range qd.Select {
		if s.CatalogEntryId != "" {
			ids = append(ids, s.CatalogEntryId)
		}
	}
	if len(ids) == 0 {
		return backend.ErrDataResponse(backend.StatusBadRequest, "no catalog entries selected")
	}

	// Fetch entries from DataCatalog
	entries, err := d.catalogClient.GetEntriesByIds(ctx, ids)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusInternal, fmt.Sprintf("fetch entries: %v", err))
	}

	// Remap non-DataBridge entries to their DataBridge counterparts (same name).
	// This handles saved queries that reference MQTT/OPC-UA entry IDs.
	entries, remappedIds := d.remapToDataBridge(ctx, entries)

	entryMap := make(map[string]*datacatalog.CatalogEntry, len(entries))
	for i := range entries {
		entryMap[entries[i].ID] = &entries[i]
	}

	// Update select items with remapped entry IDs
	if len(remappedIds) > 0 {
		for i := range qd.Select {
			if newId, ok := remappedIds[qd.Select[i].CatalogEntryId]; ok {
				qd.Select[i].CatalogEntryId = newId
				if entry, ok := entryMap[newId]; ok {
					qd.Select[i].Column = entry.GetSourceParam("column")
				}
			}
		}
	}

	// Enrich DataType from catalog entries (frontend may send empty dataType in saved queries)
	for i := range qd.Select {
		s := &qd.Select[i]
		if s.DataType == "" {
			if entry, ok := entryMap[s.CatalogEntryId]; ok {
				s.DataType = entry.DataType
			}
		}
	}

	resolveAggregations(qd.Select)

	// Group select items by (connectionUrl, database, dataset)
	targets, err := d.groupByTarget(ctx, qd.Select, entryMap)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusInternal, fmt.Sprintf("resolve targets: %v", err))
	}

	if len(targets) == 0 {
		return backend.ErrDataResponse(backend.StatusBadRequest, "no valid targets found")
	}

	d.applySafetyLimits(qd, query.TimeRange)

	// Single target — simple path (most common case)
	if len(targets) == 1 {
		t := targets[0]
		subQd := *qd
		subQd.Select = t.selectItems
		recordsQuery := buildRecordsQuery(&subQd, query.TimeRange, query.MaxDataPoints)
		dr := d.executeAndConvert(ctx, t.bridgeUrl, t.databaseName, t.datasetName, query.RefID, recordsQuery)
		if dr.Error == nil {
			for _, frame := range dr.Frames {
				d.applyDisplayNamesFromMap(frame, &subQd, entryMap)
			}
		}
		return dr
	}

	// Multiple targets — parallel execution (WHERE filters are dropped since columns may differ across datasets)
	d.logger.Info("Multi-connection query", "targets", len(targets))

	type targetResult struct {
		frames []*data.Frame
		err    error
	}

	results := make([]targetResult, len(targets))
	var wg sync.WaitGroup

	for i, t := range targets {
		wg.Add(1)
		go func(idx int, target queryTarget) {
			defer wg.Done()

			subQd := *qd
			subQd.Select = target.selectItems
			subQd.Where = nil // Drop user WHERE filters — columns may not exist in all datasets
			recordsQuery := buildRecordsQuery(&subQd, query.TimeRange, query.MaxDataPoints)

			dr := d.executeAndConvert(ctx, target.bridgeUrl, target.databaseName, target.datasetName, query.RefID, recordsQuery)
			if dr.Error != nil {
				results[idx] = targetResult{err: dr.Error}
				return
			}

			for _, frame := range dr.Frames {
				d.applyDisplayNamesFromMap(frame, &subQd, entryMap)
			}
			results[idx] = targetResult{frames: dr.Frames}
		}(i, t)
	}
	wg.Wait()

	// Collect all frames — partial failures show data from healthy targets + a notice frame
	var dr backend.DataResponse
	var errors []string

	for i, r := range results {
		if r.err != nil {
			target := targets[i]
			msg := fmt.Sprintf("%s/%s: %v", target.databaseName, target.datasetName, r.err)
			d.logger.Warn("Sub-query failed", "target", msg)
			errors = append(errors, msg)
			continue
		}
		dr.Frames = append(dr.Frames, r.frames...)
	}

	if len(errors) > 0 {
		// Set error so Grafana shows a red banner — frames are still rendered alongside
		dr.Error = fmt.Errorf("some DataBridge targets failed: %s", strings.Join(errors, "; "))
	}

	return dr
}

// groupByTarget groups select items by their DataBridge target (connection URL + database + dataset).
func (d *Datasource) groupByTarget(ctx context.Context, selectItems []models.SelectDefinition, entryMap map[string]*datacatalog.CatalogEntry) ([]queryTarget, error) {
	type targetKey struct {
		bridgeUrl    string
		databaseName string
		datasetName  string
	}

	keyToTarget := make(map[targetKey]*queryTarget)
	var orderedKeys []targetKey

	for _, s := range selectItems {
		entry, ok := entryMap[s.CatalogEntryId]
		if !ok {
			continue
		}

		connId := entry.GetSourceConnectionID()
		bridgeUrl, err := d.resolveConnectionUrl(ctx, connId)
		if err != nil {
			return nil, fmt.Errorf("resolve connection %s: %w", connId, err)
		}

		dbName := entry.GetSourceParam("database")
		if dbName == "" {
			dbName = entry.GetSourceParam("databaseName")
		}
		dsName := entry.GetSourceParam("dataset")
		if dsName == "" {
			dsName = entry.GetSourceParam("datasetName")
		}

		key := targetKey{bridgeUrl: bridgeUrl, databaseName: dbName, datasetName: dsName}
		if _, exists := keyToTarget[key]; !exists {
			keyToTarget[key] = &queryTarget{
				bridgeUrl:    bridgeUrl,
				databaseName: dbName,
				datasetName:  dsName,
			}
			orderedKeys = append(orderedKeys, key)
		}
		keyToTarget[key].selectItems = append(keyToTarget[key].selectItems, s)
	}

	result := make([]queryTarget, 0, len(orderedKeys))
	for _, key := range orderedKeys {
		result = append(result, *keyToTarget[key])
	}
	return result, nil
}

// executeAndConvert builds, executes a DataBridge query, and converts the result to a data.Frame.
func (d *Datasource) executeAndConvert(ctx context.Context, bridgeUrl, databaseName, datasetName, refID string, rq *databridge.RecordsQuery) backend.DataResponse {
	client := d.dataBridgeClient(bridgeUrl)
	resp, err := client.QueryRecords(ctx, databaseName, datasetName, rq)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusInternal, fmt.Sprintf("query %s/%s on %s: %v", databaseName, datasetName, bridgeUrl, err))
	}

	frame, err := databridge.ToDataFrame(refID, resp)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusInternal, fmt.Sprintf("convert frame: %v", err))
	}

	var dr backend.DataResponse
	dr.Frames = append(dr.Frames, frame)
	return dr
}

// applySafetyLimits enforces row limits for raw queries.
func (d *Datasource) applySafetyLimits(qd *models.QueryDefinition, timeRange backend.TimeRange) {
	if qd.OptimizeDisplay {
		return
	}
	estimatedRows := estimateRawRows(timeRange, len(qd.Select))
	if estimatedRows > int64(d.settings.HardLimitRows) {
		return // will be caught by the query builder
	}
	if estimatedRows > int64(d.settings.MaxRawRows) && qd.Limit == 0 {
		qd.Limit = d.settings.MaxRawRows
		d.logger.Warn("Auto-injecting row limit for large raw query",
			"estimatedRows", estimatedRows,
			"limit", d.settings.MaxRawRows,
		)
	}
}


// buildRecordsQuery constructs the DataBridge API query from the query definition and time range.
func buildRecordsQuery(qd *models.QueryDefinition, timeRange backend.TimeRange, maxDataPoints int64) *databridge.RecordsQuery {
	rq := &databridge.RecordsQuery{}

	// Build SELECT clause
	for _, s := range qd.Select {
		col := s.Column
		if col == "" {
			continue
		}

		agg := s.Aggregation

		// "none" = raw column, no aggregation
		if agg == "none" {
			// If time_window is active (other tags need aggregation), fallback to "last"
			if qd.OptimizeDisplay {
				agg = "last"
			} else {
				rq.Select = append(rq.Select, databridge.SelectClause{
					Column: col,
				})
				continue
			}
		}

		// Skip numeric aggregations on non-numeric types (bool, string)
		if agg != "" && !isAggregationCompatible(agg, s.DataType) {
			agg = compatibleAggregation(s.DataType)
		}

		if agg != "" && qd.OptimizeDisplay {
			params := []databridge.QueryParam{{Column: col}}
			// first/last require a second parameter (time column)
			if agg == "first" || agg == "last" {
				params = append(params, databridge.QueryParam{Column: "time"})
			}
			rq.Select = append(rq.Select, databridge.SelectClause{
				Function:   agg,
				Parameters: params,
				Alias:      aliasOrDefault(s.Alias, col+"_"+agg),
			})
		} else {
			rq.Select = append(rq.Select, databridge.SelectClause{
				Column: col,
			})
		}
	}

	// Build WHERE clause with time range
	rq.Where = buildTimeRangeWhere(timeRange, qd.Where)

	// Build GROUP BY with time_window for optimize display
	if qd.OptimizeDisplay && maxDataPoints > 0 {
		windowSeconds := timeWindowToSeconds(qd.TimeWindowInterval, qd.TimeWindowUnit)
		if windowSeconds <= 0 {
			windowSeconds = computeTimeWindow(timeRange, maxDataPoints)
		}
		if windowSeconds > 0 {
			isoDuration := formatISODuration(time.Duration(windowSeconds) * time.Second)
			twParams := []databridge.QueryParam{
				{Constant: isoDuration},
				{Column: "time"},
			}
			// time_window must appear in both SELECT and GROUP BY
			rq.Select = append(rq.Select, databridge.SelectClause{
				Function:   "time_window",
				Parameters: twParams,
				Alias:      "time",
			})
			rq.GroupBy = append(rq.GroupBy, databridge.GroupClause{
				Function:   "time_window",
				Parameters: twParams,
			})
		}
	}

	// ORDER BY — use the time alias from time_window when in optimize mode
	orderCol := qd.OrderByColumn
	orderDir := qd.OrderByDirection
	if orderCol == "" {
		orderCol = "time"
		orderDir = "asc"
	}
	rq.OrderBy = append(rq.OrderBy, databridge.OrderClause{Column: orderCol, Direction: orderDir})

	// LIMIT
	if qd.Limit > 0 {
		rq.Limit = qd.Limit
	}
	if qd.Offset > 0 {
		rq.Offset = qd.Offset
	}

	return rq
}

// buildTimeRangeWhere creates a WHERE expression combining the time range with user filters.
func buildTimeRangeWhere(timeRange backend.TimeRange, userFilter *models.FilterDefinition) *databridge.WhereExpression {
	timeConditions := []databridge.WhereExpression{
		{
			Operator: "greaterOrEqual",
			Left:     &databridge.WhereOperand{Column: "time"},
			Right:    &databridge.WhereOperand{Constant: timeRange.From.UTC().Format(time.RFC3339)},
		},
		{
			Operator: "less",
			Left:     &databridge.WhereOperand{Column: "time"},
			Right:    &databridge.WhereOperand{Constant: timeRange.To.UTC().Format(time.RFC3339)},
		},
	}

	if userFilter != nil {
		converted := convertFilter(userFilter)
		timeConditions = append(timeConditions, *converted)
	}

	return &databridge.WhereExpression{
		Operator:   "and",
		Conditions: timeConditions,
	}
}

// convertFilter recursively converts a FilterDefinition tree to a DataBridge WhereExpression.
func convertFilter(f *models.FilterDefinition) *databridge.WhereExpression {
	if f.IsLogicalGroup() {
		conditions := make([]databridge.WhereExpression, 0, len(f.Conditions))
		for i := range f.Conditions {
			conditions = append(conditions, *convertFilter(&f.Conditions[i]))
		}
		return &databridge.WhereExpression{
			Operator:   f.Operator,
			Conditions: conditions,
		}
	}

	return &databridge.WhereExpression{
		Operator: mapOperator(f.Operator),
		Left:     &databridge.WhereOperand{Column: f.Column},
		Right:    &databridge.WhereOperand{Constant: f.Value},
	}
}

// timeWindowToSeconds converts an interval + unit pair to seconds.
func timeWindowToSeconds(interval int, unit string) int64 {
	if interval <= 0 {
		return 0
	}
	switch unit {
	case "s":
		return int64(interval)
	case "m":
		return int64(interval) * 60
	case "h":
		return int64(interval) * 3600
	case "d":
		return int64(interval) * 86400
	default:
		return 0
	}
}

// computeTimeWindow calculates the optimal time window in seconds.
func computeTimeWindow(timeRange backend.TimeRange, maxDataPoints int64) int64 {
	if maxDataPoints <= 0 {
		maxDataPoints = 1000
	}

	rangeDuration := timeRange.To.Sub(timeRange.From)
	windowSeconds := int64(math.Ceil(rangeDuration.Seconds() / float64(maxDataPoints)))

	// Snap to nice intervals
	switch {
	case windowSeconds <= 1:
		return 1
	case windowSeconds <= 5:
		return 5
	case windowSeconds <= 10:
		return 10
	case windowSeconds <= 30:
		return 30
	case windowSeconds <= 60:
		return 60
	case windowSeconds <= 300:
		return 300
	case windowSeconds <= 600:
		return 600
	case windowSeconds <= 1800:
		return 1800
	case windowSeconds <= 3600:
		return 3600
	case windowSeconds <= 21600:
		return 21600
	case windowSeconds <= 43200:
		return 43200
	case windowSeconds <= 86400:
		return 86400
	default:
		return windowSeconds
	}
}

// formatDuration formats seconds into a human-readable duration string.
func formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	if hours >= 24 && hours%24 == 0 {
		return fmt.Sprintf("%d day", hours/24)
	}
	if hours > 0 {
		return fmt.Sprintf("%d hour", hours)
	}

	minutes := int(d.Minutes())
	if minutes > 0 {
		return fmt.Sprintf("%d minute", minutes)
	}

	return fmt.Sprintf("%d second", int(d.Seconds()))
}

// formatISODuration converts a duration to ISO 8601 format (e.g. PT5M, PT1H, P1D).
func formatISODuration(d time.Duration) string {
	hours := int(d.Hours())
	if hours >= 24 && hours%24 == 0 {
		return fmt.Sprintf("P%dD", hours/24)
	}
	if hours > 0 {
		return fmt.Sprintf("PT%dH", hours)
	}

	minutes := int(d.Minutes())
	if minutes > 0 {
		return fmt.Sprintf("PT%dM", minutes)
	}

	return fmt.Sprintf("PT%dS", int(d.Seconds()))
}

// estimateRawRows estimates the number of rows for a raw query (1 row/second assumption).
func estimateRawRows(timeRange backend.TimeRange, columnCount int) int64 {
	if columnCount == 0 {
		columnCount = 1
	}
	rangeSeconds := int64(timeRange.To.Sub(timeRange.From).Seconds())
	return rangeSeconds * int64(columnCount)
}

// mapOperator converts frontend operator names to DataBridge API operator names.
func mapOperator(op string) string {
	switch op {
	case "eq":
		return "equal"
	case "neq":
		return "notEqual"
	case "gt":
		return "greater"
	case "gte":
		return "greaterOrEqual"
	case "lt":
		return "less"
	case "lte":
		return "lessOrEqual"
	default:
		return op
	}
}

// isAggregationCompatible checks if the aggregation function works with the data type.
func isAggregationCompatible(agg, dataType string) bool {
	switch dataType {
	case "bool", "string":
		// Only count, first, last work on bool/string
		return agg == "count" || agg == "first" || agg == "last"
	default:
		return true
	}
}

// compatibleAggregation returns a safe fallback aggregation for non-numeric types.
func compatibleAggregation(dataType string) string {
	switch dataType {
	case "bool", "string":
		return "last"
	default:
		return "avg"
	}
}

func aliasOrDefault(alias, fallback string) string {
	if alias != "" {
		return alias
	}
	return fallback
}

// applyDisplayNamesFromMap sets display names on frame fields using a pre-built entry map.
func (d *Datasource) applyDisplayNamesFromMap(frame *data.Frame, qd *models.QueryDefinition, entryMap map[string]*datacatalog.CatalogEntry) {
	if len(qd.Select) == 0 {
		return
	}

	// Build a map of column alias -> select definition for matching fields
	selectByAlias := make(map[string]*models.SelectDefinition)
	for i := range qd.Select {
		s := &qd.Select[i]
		alias := s.Alias
		if alias == "" && s.Column != "" {
			if s.Aggregation != "" {
				alias = s.Column + "_" + s.Aggregation
			} else {
				alias = s.Column
			}
		}
		selectByAlias[alias] = s
	}

	preset := qd.DisplayNamePreset
	if preset == "" {
		preset = d.settings.DefaultDisplayName
	}
	pattern := qd.DisplayNamePattern

	// Load asset paths if needed for display name resolution
	assetPaths := d.getAssetPaths()

	for _, field := range frame.Fields {
		s, ok := selectByAlias[field.Name]
		if !ok {
			continue
		}

		entry := entryMap[s.CatalogEntryId]
		if entry == nil {
			continue
		}

		entryAssetPath := assetPaths[entry.ID]
		if entryAssetPath != "" {
			entryAssetPath = entryAssetPath + " > " + entry.Name
		}

		resolved := displayname.Resolve(preset, pattern, &displayname.ResolveContext{
			Entry:       entry,
			Column:      s.Column,
			Aggregation: s.Aggregation,
			AssetPath:   entryAssetPath,
		})

		fc := &data.FieldConfig{
			DisplayNameFromDS: resolved,
			Description:       buildFieldDescription(entry),
		}

		// Push catalog metadata into Grafana field config (unit, min, max, decimals, thresholds)
		if entry.Metadata != nil {
			if entry.Metadata.Unit != "" {
				fc.Unit = entry.Metadata.Unit
			}
			if entry.Metadata.Min.Value != nil {
				fc.SetMin(*entry.Metadata.Min.Value)
			}
			if entry.Metadata.Max.Value != nil {
				fc.SetMax(*entry.Metadata.Max.Value)
			}
			if entry.Metadata.Decimals.Value != nil {
				fc.SetDecimals(uint16(*entry.Metadata.Decimals.Value))
			}
			// Build thresholds from min/max for visual display in gauges and panels
			if entry.Metadata.Min.Value != nil && entry.Metadata.Max.Value != nil {
				fc.Thresholds = &data.ThresholdsConfig{
					Mode: data.ThresholdsModeAbsolute,
					Steps: []data.Threshold{
						data.NewThreshold(*entry.Metadata.Min.Value, "green", ""),
						data.NewThreshold(*entry.Metadata.Max.Value, "red", ""),
					},
				}
			}
		}

		field.Config = fc
	}
}

// buildFieldDescription builds a human-readable description from catalog entry metadata.
func buildFieldDescription(entry *datacatalog.CatalogEntry) string {
	if entry == nil {
		return ""
	}

	var parts []string

	// Description (prefer English)
	if entry.Metadata != nil {
		if descMap := entry.Metadata.Description(); descMap != nil {
			if desc, ok := descMap["en-US"]; ok && desc != "" {
				parts = append(parts, desc)
			} else if desc, ok := descMap["de-DE"]; ok && desc != "" {
				parts = append(parts, desc)
			}
		}
	}

	// Tag level 1
	if entry.Metadata != nil && entry.Metadata.TagLevel1 != "" {
		parts = append(parts, fmt.Sprintf("Tag: %s", entry.Metadata.TagLevel1))
	}

	// Source connection
	if entry.SourceConnection != nil && entry.SourceConnection.Name != "" {
		parts = append(parts, fmt.Sprintf("Source: %s", entry.SourceConnection.Name))
	}

	// Range
	if entry.Metadata != nil && entry.Metadata.Min.Value != nil && entry.Metadata.Max.Value != nil {
		unit := ""
		if entry.Metadata.Unit != "" {
			unit = " " + entry.Metadata.Unit
		}
		parts = append(parts, fmt.Sprintf("Range: %g – %g%s", *entry.Metadata.Min.Value, *entry.Metadata.Max.Value, unit))
	}

	return strings.Join(parts, " | ")
}

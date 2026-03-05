package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
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

	// Resolve the DataBridge URL based on mode
	var (
		bridgeUrl    string
		databaseName string
		datasetName  string
		err          error
	)

	switch qd.Mode {
	case "raw":
		bridgeUrl, err = d.resolveConnectionUrl(ctx, qd.ConnectionId)
		if err != nil {
			return backend.ErrDataResponse(backend.StatusInternal, fmt.Sprintf("resolve connection: %v", err))
		}
		databaseName = qd.DatabaseName
		datasetName = qd.DatasetName

	case "dataCatalog":
		bridgeUrl, databaseName, datasetName, err = d.resolveFromCatalog(ctx, &qd)
		if err != nil {
			return backend.ErrDataResponse(backend.StatusInternal, fmt.Sprintf("resolve catalog: %v", err))
		}

	default:
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("unknown mode: %s", qd.Mode))
	}

	if databaseName == "" || datasetName == "" {
		return backend.ErrDataResponse(backend.StatusBadRequest, "database and dataset are required")
	}

	// Enforce safety limits for raw queries
	if !qd.OptimizeDisplay {
		estimatedRows := estimateRawRows(query.TimeRange, len(qd.Select))
		if estimatedRows > int64(d.settings.HardLimitRows) {
			return backend.ErrDataResponse(
				backend.StatusBadRequest,
				fmt.Sprintf("Query blocked: estimated %d rows exceeds hard limit of %d. Use Optimize Display.", estimatedRows, d.settings.HardLimitRows),
			)
		}
		if estimatedRows > int64(d.settings.MaxRawRows) && qd.Limit == 0 {
			qd.Limit = d.settings.MaxRawRows
			d.logger.Warn("Auto-injecting row limit for large raw query",
				"estimatedRows", estimatedRows,
				"limit", d.settings.MaxRawRows,
			)
		}
	}

	// Build the DataBridge query
	recordsQuery := buildRecordsQuery(&qd, query.TimeRange, query.MaxDataPoints)

	// Execute query
	client := d.dataBridgeClient(bridgeUrl)
	resp, err := client.QueryRecords(ctx, databaseName, datasetName, recordsQuery)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusInternal, fmt.Sprintf("query records: %v", err))
	}

	// Convert response to data.Frame
	frame, err := databridge.ToDataFrame(query.RefID, resp)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusInternal, fmt.Sprintf("convert frame: %v", err))
	}

	// Apply display names from catalog entries
	if qd.Mode == "dataCatalog" {
		d.applyDisplayNames(ctx, frame, &qd)
	}

	var dr backend.DataResponse
	dr.Frames = append(dr.Frames, frame)
	return dr
}

// resolveFromCatalog determines the DataBridge URL, database, and dataset from catalog entries.
func (d *Datasource) resolveFromCatalog(ctx context.Context, qd *models.QueryDefinition) (string, string, string, error) {
	if d.catalogClient == nil {
		return "", "", "", fmt.Errorf("DataCatalog URL is not configured")
	}

	if len(qd.CatalogEntryIds) == 0 && len(qd.Select) == 0 {
		return "", "", "", fmt.Errorf("no catalog entries selected")
	}

	// Collect entry IDs from both catalogEntryIds and select definitions
	ids := make([]string, 0)
	ids = append(ids, qd.CatalogEntryIds...)
	for _, s := range qd.Select {
		if s.CatalogEntryId != "" {
			ids = append(ids, s.CatalogEntryId)
		}
	}

	if len(ids) == 0 {
		return "", "", "", fmt.Errorf("no catalog entry IDs found")
	}

	entries, err := d.catalogClient.GetEntriesByIds(ctx, ids)
	if err != nil {
		return "", "", "", fmt.Errorf("fetch entries: %w", err)
	}

	if len(entries) == 0 {
		return "", "", "", fmt.Errorf("no catalog entries found for IDs: %v", ids)
	}

	// Resolve the connection URL from the first entry's source connection
	connectionUrl, err := d.resolveConnectionUrl(ctx, entries[0].SourceConnectionID)
	if err != nil {
		return "", "", "", fmt.Errorf("resolve connection for entry: %w", err)
	}

	// Get database and dataset from source params
	databaseName := entries[0].SourceParams["databaseName"]
	datasetName := entries[0].SourceParams["datasetName"]

	return connectionUrl, databaseName, datasetName, nil
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
		if agg == "" {
			agg = qd.Aggregation
		}

		if agg != "" && qd.OptimizeDisplay {
			rq.Select = append(rq.Select, databridge.SelectClause{
				Function:   agg,
				Parameters: []databridge.ColumnRef{{Column: col}},
				Alias:      aliasOrDefault(s.Alias, col+"_"+agg),
			})
		} else {
			rq.Select = append(rq.Select, databridge.SelectClause{
				Column: &databridge.ColumnRef{Column: col},
			})
		}
	}

	// Build WHERE clause with time range
	rq.Where = buildTimeRangeWhere(timeRange, qd.Where)

	// Build GROUP BY with time_window for optimize display
	if qd.OptimizeDisplay && maxDataPoints > 0 {
		windowSeconds := computeTimeWindow(timeRange, maxDataPoints)
		if windowSeconds > 0 {
			windowStr := formatDuration(time.Duration(windowSeconds) * time.Second)
			rq.GroupBy = append(rq.GroupBy, databridge.GroupClause{
				Function:   "time_window",
				Parameters: []interface{}{windowStr, "time"},
				Alias:      "time",
			})
		}
	}

	// ORDER BY time ASC by default
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

// buildTimeRangeWhere creates a WHERE expression for the time range.
func buildTimeRangeWhere(timeRange backend.TimeRange, userConditions []models.WhereCondition) *databridge.WhereExpression {
	conditions := []databridge.WhereCondition{
		{
			Operator: "gte",
			Left:     &databridge.WhereOperand{Column: "time"},
			Right:    &databridge.WhereOperand{Constant: timeRange.From.UTC().Format(time.RFC3339)},
		},
		{
			Operator: "lt",
			Left:     &databridge.WhereOperand{Column: "time"},
			Right:    &databridge.WhereOperand{Constant: timeRange.To.UTC().Format(time.RFC3339)},
		},
	}

	for _, uc := range userConditions {
		conditions = append(conditions, databridge.WhereCondition{
			Operator: uc.Operator,
			Left:     &databridge.WhereOperand{Column: uc.Column},
			Right:    &databridge.WhereOperand{Constant: uc.Value},
		})
	}

	return &databridge.WhereExpression{
		Operator:   "and",
		Conditions: conditions,
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

// formatDuration formats seconds into a DataBridge-compatible duration string.
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

// estimateRawRows estimates the number of rows for a raw query (1 row/second assumption).
func estimateRawRows(timeRange backend.TimeRange, columnCount int) int64 {
	if columnCount == 0 {
		columnCount = 1
	}
	rangeSeconds := int64(timeRange.To.Sub(timeRange.From).Seconds())
	return rangeSeconds * int64(columnCount)
}

func aliasOrDefault(alias, fallback string) string {
	if alias != "" {
		return alias
	}
	return fallback
}

// applyDisplayNames sets display names on frame fields based on catalog entries.
func (d *Datasource) applyDisplayNames(ctx context.Context, frame *data.Frame, qd *models.QueryDefinition) {
	if d.catalogClient == nil || len(qd.Select) == 0 {
		return
	}

	// Collect all catalog entry IDs
	ids := make([]string, 0)
	for _, s := range qd.Select {
		if s.CatalogEntryId != "" {
			ids = append(ids, s.CatalogEntryId)
		}
	}
	if len(ids) == 0 {
		return
	}

	// Fetch entries (may be cached by the catalog client)
	entries, err := d.catalogClient.GetEntriesByIds(ctx, ids)
	if err != nil {
		d.logger.Warn("Failed to fetch entries for display names", "error", err)
		return
	}

	entryMap := make(map[string]*datacatalog.CatalogEntry, len(entries))
	for i := range entries {
		entryMap[entries[i].ID] = &entries[i]
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

	for _, field := range frame.Fields {
		s, ok := selectByAlias[field.Name]
		if !ok {
			continue
		}

		entry := entryMap[s.CatalogEntryId]
		if entry == nil {
			continue
		}

		resolved := displayname.Resolve(preset, pattern, &displayname.ResolveContext{
			Entry:       entry,
			Column:      s.Column,
			Aggregation: s.Aggregation,
		})

		field.Config = &data.FieldConfig{
			DisplayNameFromDS: resolved,
		}
	}
}

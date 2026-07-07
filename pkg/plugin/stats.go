package plugin

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"

	"github.com/industream/industream-data-bridge/pkg/datacatalog"
	"github.com/industream/industream-data-bridge/pkg/databridge"
	"github.com/industream/industream-data-bridge/pkg/displayname"
	"github.com/industream/industream-data-bridge/pkg/models"
)

// DefaultStats is applied when the stats strategy is used without picking any stat.
var DefaultStats = []string{"mean", "min", "max", "p50"}

// statsRow holds one signal's computed statistics, keyed by stat name.
type statsRow struct {
	signal string
	values map[string]float64
}

// buildStatsQuery builds the DataBridge POST /records/stats request. An "entry" is a
// signal = a column/_field (see DataBridge fix #96), so the selected columns are passed
// as entries verbatim. maxSamples is left at the API default (a true cap post-fix).
func buildStatsQuery(columns []string, compute []string, timeRange backend.TimeRange) *databridge.StatsQuery {
	if len(compute) == 0 {
		compute = DefaultStats
	}
	return &databridge.StatsQuery{
		Entries: columns,
		Start:   timeRange.From.UTC().Format(time.RFC3339),
		End:     timeRange.To.UTC().Format(time.RFC3339),
		Compute: compute,
	}
}

// statsToFrame builds a table frame: a "Signal" string column plus one Float64 column
// per stat, in statOrder. A stat missing for a signal is left null. Suited for a Stat
// or Table panel (one row per signal, a scalar per stat over the panel range).
func statsToFrame(refID string, statOrder []string, rows []statsRow) *data.Frame {
	signalField := data.NewField("Signal", nil, make([]string, len(rows)))

	statFields := make([]*data.Field, len(statOrder))
	for i, stat := range statOrder {
		statFields[i] = data.NewField(stat, nil, make([]*float64, len(rows)))
	}

	for rowIdx, row := range rows {
		signalField.Set(rowIdx, row.signal)
		for i, stat := range statOrder {
			if value, ok := row.values[stat]; ok {
				v := value
				statFields[i].Set(rowIdx, &v)
			}
		}
	}

	fields := append([]*data.Field{signalField}, statFields...)
	return data.NewFrame(refID, fields...)
}

// handleStatsQuery computes scalar statistics per selected signal via /records/stats
// and returns one table frame (a "Signal" column + one column per stat). Works in both
// dataCatalog and raw mode; a signal is addressed by its column/_field.
func (d *Datasource) handleStatsQuery(ctx context.Context, query backend.DataQuery, qd *models.QueryDefinition) backend.DataResponse {
	if len(qd.Select) == 0 {
		return backend.ErrDataResponse(backend.StatusBadRequest, "no signals selected for statistics")
	}

	statOrder := qd.Stats
	if len(statOrder) == 0 {
		statOrder = DefaultStats
	}

	if qd.Mode == "raw" {
		return d.handleRawStatsQuery(ctx, query, qd, statOrder)
	}
	return d.handleCatalogStatsQuery(ctx, query, qd, statOrder)
}

// handleRawStatsQuery computes stats for raw mode: a single connection + database,
// with the selected columns used directly as signal entries.
func (d *Datasource) handleRawStatsQuery(ctx context.Context, query backend.DataQuery, qd *models.QueryDefinition, statOrder []string) backend.DataResponse {
	bridgeUrl, err := d.resolveConnectionUrl(ctx, qd.ConnectionId)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusInternal, fmt.Sprintf("resolve connection: %v", err))
	}
	if qd.DatabaseName == "" {
		return backend.ErrDataResponse(backend.StatusBadRequest, "database is required")
	}

	columns := make([]string, 0, len(qd.Select))
	names := make(map[string]string, len(qd.Select))
	for i := range qd.Select {
		s := &qd.Select[i]
		if s.Column == "" {
			continue
		}
		columns = append(columns, s.Column)
		names[s.Column] = aliasOrDefault(s.Alias, s.Column)
	}

	rows, err := d.computeStatsRows(ctx, bridgeUrl, qd.DatabaseName, columns, statOrder, query.TimeRange, names)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusInternal, err.Error())
	}

	var dr backend.DataResponse
	dr.Frames = append(dr.Frames, statsToFrame(query.RefID, statOrder, rows))
	return dr
}

// handleCatalogStatsQuery resolves the selected catalog entries, groups them by
// DataBridge target, and runs one stats call per target (columns are the signals).
func (d *Datasource) handleCatalogStatsQuery(ctx context.Context, query backend.DataQuery, qd *models.QueryDefinition, statOrder []string) backend.DataResponse {
	if d.catalogClient == nil {
		return backend.ErrDataResponse(backend.StatusInternal, "DataCatalog URL is not configured")
	}

	ids := make([]string, 0, len(qd.Select))
	for _, s := range qd.Select {
		if s.CatalogEntryId != "" {
			ids = append(ids, s.CatalogEntryId)
		}
	}

	entries, err := d.getEntriesByIds(ctx, ids)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusInternal, fmt.Sprintf("fetch entries: %v", err))
	}

	entries, remappedIds := d.remapToDataBridge(ctx, entries)
	entryMap := make(map[string]*datacatalog.CatalogEntry, len(entries))
	for i := range entries {
		entryMap[entries[i].ID] = &entries[i]
	}
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

	var byColumn map[string]*datacatalog.CatalogEntry
	if selectsNeedColumnFallback(qd.Select, entryMap) {
		byColumn = d.dataBridgeEntriesByColumn(ctx)
	}

	targets, err := d.groupByTarget(ctx, qd.Select, entryMap, byColumn)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusInternal, fmt.Sprintf("resolve targets: %v", err))
	}
	if len(targets) == 0 {
		return backend.ErrDataResponse(backend.StatusBadRequest, "no valid targets found")
	}

	preset := qd.DisplayNamePreset
	if preset == "" {
		preset = d.settings.DefaultDisplayName
	}

	var allRows []statsRow
	var errs []string
	for _, t := range targets {
		columns := make([]string, 0, len(t.selectItems))
		names := make(map[string]string, len(t.selectItems))
		for i := range t.selectItems {
			s := &t.selectItems[i]
			if s.Column == "" {
				continue
			}
			columns = append(columns, s.Column)
			names[s.Column] = d.statsDisplayName(preset, qd.DisplayNamePattern, s, entryMap, byColumn)
		}

		rows, err := d.computeStatsRows(ctx, t.bridgeUrl, t.databaseName, columns, statOrder, query.TimeRange, names)
		if err != nil {
			d.logger.Warn("Stats sub-query failed", "database", t.databaseName, "error", err)
			errs = append(errs, err.Error())
			continue
		}
		allRows = append(allRows, rows...)
	}

	// All targets failed — surface the error instead of an empty frame.
	if len(allRows) == 0 && len(errs) > 0 {
		return backend.ErrDataResponse(backend.StatusInternal, strings.Join(errs, "; "))
	}

	var dr backend.DataResponse
	dr.Frames = append(dr.Frames, statsToFrame(query.RefID, statOrder, allRows))
	if len(errs) > 0 {
		// Partial failure: render healthy signals and flag the rest.
		dr.Error = fmt.Errorf("some stats targets failed: %s", strings.Join(errs, "; "))
	}
	return dr
}

// computeStatsRows runs one /records/stats call and maps the response to rows, one per
// requested column, using the provided display names (falling back to the column).
func (d *Datasource) computeStatsRows(ctx context.Context, bridgeUrl, databaseName string, columns, compute []string, timeRange backend.TimeRange, displayNames map[string]string) ([]statsRow, error) {
	if len(columns) == 0 {
		return nil, nil
	}

	client := d.dataBridgeClient(bridgeUrl)
	resp, err := client.ComputeStats(ctx, databaseName, buildStatsQuery(columns, compute, timeRange))
	if err != nil {
		return nil, fmt.Errorf("stats %s on %s: %w", databaseName, bridgeUrl, err)
	}

	rows := make([]statsRow, 0, len(columns))
	for _, column := range columns {
		name := displayNames[column]
		if name == "" {
			name = column
		}
		rows = append(rows, statsRow{signal: name, values: resp[column]})
	}
	return rows, nil
}

// statsDisplayName resolves a signal's display name from its catalog entry, falling
// back to the raw column when the entry cannot be resolved.
func (d *Datasource) statsDisplayName(preset, pattern string, s *models.SelectDefinition, entryMap, byColumn map[string]*datacatalog.CatalogEntry) string {
	entry := resolveEntry(s, entryMap, byColumn)
	if entry == nil {
		return s.Column
	}
	name := displayname.Resolve(preset, pattern, &displayname.ResolveContext{
		Entry:  entry,
		Column: s.Column,
	})
	if name == "" {
		return s.Column
	}
	return name
}

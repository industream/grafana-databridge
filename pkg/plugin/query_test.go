package plugin

import (
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"

	"github.com/industream/industream-data-bridge/pkg/databridge"
	"github.com/industream/industream-data-bridge/pkg/models"
)

func TestComputeTimeWindow(t *testing.T) {
	tests := []struct {
		name          string
		rangeDuration time.Duration
		maxDataPoints int64
		expected      int64
	}{
		{"1 hour with 1000 points", time.Hour, 1000, 5},
		{"1 day with 1000 points", 24 * time.Hour, 1000, 300},
		{"1 week with 1000 points", 7 * 24 * time.Hour, 1000, 1800},
		{"1 month with 1000 points", 30 * 24 * time.Hour, 1000, 3600},
		{"6 months with 1000 points", 180 * 24 * time.Hour, 1000, 21600},
		{"1 year with 1000 points", 365 * 24 * time.Hour, 1000, 43200},
		{"5 minutes with 500 points", 5 * time.Minute, 500, 1},
		{"default maxDataPoints", time.Hour, 0, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Now()
			tr := backend.TimeRange{From: now.Add(-tt.rangeDuration), To: now}
			result := computeTimeWindow(tr, tt.maxDataPoints)
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		input    time.Duration
		expected string
	}{
		{time.Second, "1 second"},
		{5 * time.Second, "5 second"},
		{time.Minute, "1 minute"},
		{5 * time.Minute, "5 minute"},
		{time.Hour, "1 hour"},
		{6 * time.Hour, "6 hour"},
		{24 * time.Hour, "1 day"},
		{7 * 24 * time.Hour, "7 day"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatDuration(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestBuildRecordsQuery_VarianceMapsToVar(t *testing.T) {
	qd := &models.QueryDefinition{
		Mode:            "raw",
		Strategy:        "timeseries",
		OptimizeDisplay: true,
		Select:          []models.SelectDefinition{{Column: "t", Aggregation: "variance"}},
	}
	now := time.Now()
	rq := buildRecordsQuery(qd, backend.TimeRange{From: now.Add(-time.Hour), To: now}, 1000)

	// DataBridge knows the function as "var", not "variance".
	if rq.Select[0].Function != "var" {
		t.Errorf("variance should map to var, got %q", rq.Select[0].Function)
	}
}

func TestBuildRecordsQuery_TimePositionalAggregatesGetTimeParam(t *testing.T) {
	// first/last (value) and *_at (time) all require a second [time] parameter.
	for _, agg := range []string{"first", "last", "first_at", "last_at", "min_at", "max_at"} {
		qd := &models.QueryDefinition{
			Mode:            "raw",
			Strategy:        "timeseries",
			OptimizeDisplay: true,
			Select:          []models.SelectDefinition{{Column: "t", Aggregation: agg}},
		}
		now := time.Now()
		rq := buildRecordsQuery(qd, backend.TimeRange{From: now.Add(-time.Hour), To: now}, 1000)

		params := rq.Select[0].Parameters
		if len(params) != 2 || params[0].Column != "t" || params[1].Column != "time" {
			t.Errorf("%s: expected params [t, time], got %+v", agg, params)
		}
	}
}

func TestBuildRecordsQuery_OptimizeDisplay(t *testing.T) {
	qd := &models.QueryDefinition{
		Mode:            "raw",
		Strategy:        "timeseries",
		OptimizeDisplay: true,
		Select: []models.SelectDefinition{
			{Column: "temperature", Aggregation: "avg"},
			{Column: "humidity", Aggregation: "max"},
		},
	}

	now := time.Now()
	tr := backend.TimeRange{From: now.Add(-time.Hour), To: now}

	rq := buildRecordsQuery(qd, tr, 1000)

	// 2 data columns + 1 time_window = 3 select clauses
	if len(rq.Select) != 3 {
		t.Fatalf("expected 3 select clauses (2 data + 1 time_window), got %d", len(rq.Select))
	}

	// First column should use per-tag aggregation
	if rq.Select[0].Function != "avg" {
		t.Errorf("expected function avg, got %s", rq.Select[0].Function)
	}

	// Second column should use its own aggregation
	if rq.Select[1].Function != "max" {
		t.Errorf("expected function max, got %s", rq.Select[1].Function)
	}

	// Third select should be time_window
	if rq.Select[2].Function != "time_window" {
		t.Errorf("expected time_window in select, got %s", rq.Select[2].Function)
	}

	// Should have time_window GROUP BY
	if len(rq.GroupBy) != 1 {
		t.Fatalf("expected 1 group by, got %d", len(rq.GroupBy))
	}
	if rq.GroupBy[0].Function != "time_window" {
		t.Errorf("expected time_window function, got %s", rq.GroupBy[0].Function)
	}

	// Should have ORDER BY time ASC
	if len(rq.OrderBy) != 1 || rq.OrderBy[0].Column != "time" || rq.OrderBy[0].Direction != "asc" {
		t.Errorf("expected ORDER BY time asc, got %+v", rq.OrderBy)
	}
}

func TestBuildRecordsQuery_TableStrategyReducesOverRange_NoTimeWindow(t *testing.T) {
	qd := &models.QueryDefinition{
		Mode:            "raw",
		Strategy:        "table",
		OptimizeDisplay: true,
		Select: []models.SelectDefinition{
			{Column: "temperature", Aggregation: "count"},
			{Column: "humidity", Aggregation: "avg"},
		},
	}
	now := time.Now()
	rq := buildRecordsQuery(qd, backend.TimeRange{From: now.Add(-time.Hour), To: now}, 1000)

	// Table strategy: one aggregate per signal over the whole range, NO time_window
	// bucketing (so count/sum are totals over the period, not per-bucket values).
	if len(rq.GroupBy) != 0 {
		t.Errorf("table strategy must not group by time_window, got %+v", rq.GroupBy)
	}
	if len(rq.Select) != 2 {
		t.Fatalf("expected 2 aggregate selects (no time_window), got %d", len(rq.Select))
	}
	for _, sc := range rq.Select {
		if sc.Function == "time_window" {
			t.Errorf("table strategy must not add a time_window select")
		}
	}
	if rq.Select[0].Function != "count" || rq.Select[1].Function != "avg" {
		t.Errorf("expected count/avg, got %s/%s", rq.Select[0].Function, rq.Select[1].Function)
	}
}

func TestBuildRecordsQuery_RawMode(t *testing.T) {
	qd := &models.QueryDefinition{
		Mode:            "raw",
		Strategy:        "timeseries",
		OptimizeDisplay: false,
		Select: []models.SelectDefinition{
			{Column: "temperature"},
		},
		Limit: 1000,
	}

	now := time.Now()
	tr := backend.TimeRange{From: now.Add(-time.Hour), To: now}

	rq := buildRecordsQuery(qd, tr, 1000)

	// Raw mode should not have GROUP BY
	if len(rq.GroupBy) != 0 {
		t.Errorf("expected no group by in raw mode, got %d", len(rq.GroupBy))
	}

	// Should have LIMIT
	if rq.Limit != 1000 {
		t.Errorf("expected limit 1000, got %d", rq.Limit)
	}

	// Select should be plain column (no function)
	if rq.Select[0].Function != "" {
		t.Errorf("expected no function in raw mode, got %s", rq.Select[0].Function)
	}
	if rq.Select[0].Column != "temperature" {
		t.Errorf("expected column 'temperature', got %s", rq.Select[0].Column)
	}
}

func TestBuildRecordsQuery_TransformsForwarded(t *testing.T) {
	qd := &models.QueryDefinition{
		Mode:     "raw",
		Strategy: "timeseries",
		Select: []models.SelectDefinition{
			{Column: "temperature"},
		},
		Transforms: []databridge.Transform{
			{MovingAverage: &databridge.MovingAverageParams{Window: 5}},
			{CumulativeSum: &databridge.CumulativeSumParams{}},
		},
	}

	now := time.Now()
	tr := backend.TimeRange{From: now.Add(-time.Hour), To: now}

	rq := buildRecordsQuery(qd, tr, 1000)

	if len(rq.Transforms) != 2 {
		t.Fatalf("expected 2 transforms forwarded, got %d", len(rq.Transforms))
	}
	if rq.Transforms[0].MovingAverage == nil || rq.Transforms[0].MovingAverage.Window != 5 {
		t.Errorf("expected movingAverage window 5, got %+v", rq.Transforms[0])
	}
	if rq.Transforms[1].CumulativeSum == nil {
		t.Errorf("expected cumulativeSum transform, got %+v", rq.Transforms[1])
	}
}

func TestBuildRecordsQuery_ResampleSuppressesTimeWindow(t *testing.T) {
	qd := &models.QueryDefinition{
		Mode:            "raw",
		Strategy:        "timeseries",
		OptimizeDisplay: true, // would normally inject time_window
		Select: []models.SelectDefinition{
			{Column: "temperature", Aggregation: "avg"},
		},
		Transforms: []databridge.Transform{
			{Resample: &databridge.ResampleParams{Every: "PT1M", Aggregation: "mean"}},
		},
	}

	now := time.Now()
	tr := backend.TimeRange{From: now.Add(-time.Hour), To: now}

	rq := buildRecordsQuery(qd, tr, 1000)

	// No auto time_window in SELECT when a resample is configured.
	for _, s := range rq.Select {
		if s.Function == "time_window" {
			t.Errorf("expected no time_window in SELECT when resample configured, got %+v", rq.Select)
		}
	}
	// No time_window GROUP BY either.
	if len(rq.GroupBy) != 0 {
		t.Errorf("expected no GROUP BY when resample configured, got %+v", rq.GroupBy)
	}
	// Resample transform is forwarded.
	if len(rq.Transforms) != 1 || rq.Transforms[0].Resample == nil {
		t.Fatalf("expected resample transform forwarded, got %+v", rq.Transforms)
	}
}

func TestBuildRecordsQuery_NoResampleKeepsTimeWindow(t *testing.T) {
	qd := &models.QueryDefinition{
		Mode:            "raw",
		Strategy:        "timeseries",
		OptimizeDisplay: true,
		Select: []models.SelectDefinition{
			{Column: "temperature", Aggregation: "avg"},
		},
		Transforms: []databridge.Transform{
			{Fill: &databridge.FillParams{Method: "linear"}},
		},
	}

	now := time.Now()
	tr := backend.TimeRange{From: now.Add(-time.Hour), To: now}

	rq := buildRecordsQuery(qd, tr, 1000)

	// time_window must still be injected (regression guard) when no resample.
	if len(rq.GroupBy) != 1 || rq.GroupBy[0].Function != "time_window" {
		t.Errorf("expected time_window GROUP BY without resample, got %+v", rq.GroupBy)
	}
}

func TestBuildTimeRangeWhere(t *testing.T) {
	now := time.Now()
	tr := backend.TimeRange{From: now.Add(-time.Hour), To: now}

	where := buildTimeRangeWhere(tr, nil)

	if where.Operator != "and" {
		t.Errorf("expected 'and' operator, got %s", where.Operator)
	}
	if len(where.Conditions) != 2 {
		t.Fatalf("expected 2 conditions, got %d", len(where.Conditions))
	}
	if where.Conditions[0].Operator != "greaterOrEqual" {
		t.Errorf("expected 'greaterOrEqual' for first condition, got %s", where.Conditions[0].Operator)
	}
	if where.Conditions[1].Operator != "less" {
		t.Errorf("expected 'less' for second condition, got %s", where.Conditions[1].Operator)
	}
}

func TestBuildTimeRangeWhere_WithUserFilter(t *testing.T) {
	now := time.Now()
	tr := backend.TimeRange{From: now.Add(-time.Hour), To: now}

	userFilter := &models.FilterDefinition{
		Operator: "and",
		Conditions: []models.FilterDefinition{
			{Column: "temperature", Operator: "gt", Value: 25.0},
			{Column: "humidity", Operator: "lt", Value: 80.0},
		},
	}

	where := buildTimeRangeWhere(tr, userFilter)

	// 2 time conditions + 1 user group = 3
	if len(where.Conditions) != 3 {
		t.Fatalf("expected 3 conditions (2 time + 1 user group), got %d", len(where.Conditions))
	}
	// The user group should be an AND with 2 sub-conditions
	userGroup := where.Conditions[2]
	if userGroup.Operator != "and" {
		t.Errorf("expected 'and' for user group, got %s", userGroup.Operator)
	}
	if len(userGroup.Conditions) != 2 {
		t.Fatalf("expected 2 sub-conditions, got %d", len(userGroup.Conditions))
	}
	if userGroup.Conditions[0].Operator != "greater" {
		t.Errorf("expected 'greater', got %s", userGroup.Conditions[0].Operator)
	}
}

func TestBuildTimeRangeWhere_WithNestedOrFilter(t *testing.T) {
	now := time.Now()
	tr := backend.TimeRange{From: now.Add(-time.Hour), To: now}

	userFilter := &models.FilterDefinition{
		Operator: "or",
		Conditions: []models.FilterDefinition{
			{Column: "temperature", Operator: "gt", Value: 30.0},
			{Column: "pressure", Operator: "lt", Value: 1000.0},
		},
	}

	where := buildTimeRangeWhere(tr, userFilter)

	if len(where.Conditions) != 3 {
		t.Fatalf("expected 3, got %d", len(where.Conditions))
	}
	orGroup := where.Conditions[2]
	if orGroup.Operator != "or" {
		t.Errorf("expected 'or', got %s", orGroup.Operator)
	}
}

func TestBuildTimeRangeWhere_SingleCondition(t *testing.T) {
	now := time.Now()
	tr := backend.TimeRange{From: now.Add(-time.Hour), To: now}

	userFilter := &models.FilterDefinition{
		Column:   "temperature",
		Operator: "gt",
		Value:    25.0,
	}

	where := buildTimeRangeWhere(tr, userFilter)

	if len(where.Conditions) != 3 {
		t.Fatalf("expected 3, got %d", len(where.Conditions))
	}
	if where.Conditions[2].Operator != "greater" {
		t.Errorf("expected 'greater', got %s", where.Conditions[2].Operator)
	}
}

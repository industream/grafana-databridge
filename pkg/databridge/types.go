package databridge

// DatabaseInfo represents a database from the DataBridge API.
type DatabaseInfo struct {
	Name string `json:"name"`
}

// DatasetInfo represents a dataset from the DataBridge API.
type DatasetInfo struct {
	Name    string       `json:"name"`
	Columns []ColumnInfo `json:"columns,omitempty"`
}

// PaginatedResponse wraps items returned by the DataBridge API.
type PaginatedResponse[T any] struct {
	Items []T `json:"items"`
}

// DatasetSchema holds column information for a dataset.
type DatasetSchema struct {
	Columns []ColumnInfo `json:"columns"`
}

// ColumnInfo describes a single column.
type ColumnInfo struct {
	Name     string `json:"name"`
	DataType string `json:"type"`
	Indexed  bool   `json:"indexed"`
}

// RecordsQuery is the request body for POST /records/query.
type RecordsQuery struct {
	Select     []SelectClause   `json:"select,omitempty"`
	Where      *WhereExpression `json:"where,omitempty"`
	GroupBy    []GroupClause    `json:"groupBy,omitempty"`
	OrderBy    []OrderClause    `json:"orderBy,omitempty"`
	Limit      int              `json:"limit,omitempty"`
	Offset     int              `json:"offset,omitempty"`
	Transforms []Transform      `json:"transforms,omitempty"`
}

// Transform is a single query-time transform applied by DataBridge after the
// query, in pipeline order. It uses the wrapper-object shape of the DataBridge
// API: exactly one field is set per transform (e.g. {"resample": {...}}).
// Nillable pointers let the backend detect which transform is present.
type Transform struct {
	Resample      *ResampleParams      `json:"resample,omitempty"`
	Fill          *FillParams          `json:"fill,omitempty"`
	MovingAverage *MovingAverageParams `json:"movingAverage,omitempty"`
	CumulativeSum *CumulativeSumParams `json:"cumulativeSum,omitempty"`
	RollingStats  *RollingStatsParams  `json:"rollingStats,omitempty"`
}

// ResampleParams buckets records into fixed time intervals and aggregates them.
type ResampleParams struct {
	Every       string `json:"every"`
	Aggregation string `json:"aggregation,omitempty"`
	CreateEmpty bool   `json:"createEmpty,omitempty"`
	Offset      string `json:"offset,omitempty"`
}

// FillParams fills gaps in the series. Value is a pointer so a zero fill value
// is only sent when explicitly set.
type FillParams struct {
	Method string   `json:"method,omitempty"`
	Value  *float64 `json:"value,omitempty"`
	Limit  int      `json:"limit,omitempty"`
}

// MovingAverageParams smooths each numeric column over a sliding window.
type MovingAverageParams struct {
	Window int `json:"window"`
}

// CumulativeSumParams accumulates each numeric column. It takes no parameters.
type CumulativeSumParams struct{}

// RollingStatsParams computes sliding-window statistics per numeric column.
type RollingStatsParams struct {
	Window       int      `json:"window"`
	Stats        []string `json:"stats,omitempty"`
	OutputSuffix bool     `json:"outputSuffix,omitempty"`
}

// SelectClause represents a column selection with optional aggregation.
type SelectClause struct {
	Column     string       `json:"column,omitempty"`
	Function   string       `json:"function,omitempty"`
	Parameters []QueryParam `json:"parameters,omitempty"`
	Alias      string       `json:"alias,omitempty"`
}

// QueryParam is a polymorphic parameter that can be a column reference or a constant.
type QueryParam struct {
	Column   string      `json:"column,omitempty"`
	Constant interface{} `json:"constant,omitempty"`
}

// GroupClause represents a GROUP BY expression.
type GroupClause struct {
	Column     string       `json:"column,omitempty"`
	Function   string       `json:"function,omitempty"`
	Parameters []QueryParam `json:"parameters,omitempty"`
	Alias      string       `json:"alias,omitempty"`
}

// OrderClause represents an ORDER BY expression.
type OrderClause struct {
	Column    string `json:"column"`
	Direction string `json:"direction"`
}

// WhereExpression is a boolean expression tree for the WHERE clause.
// It can be a logical group (and/or with sub-conditions) or a leaf comparison.
type WhereExpression struct {
	Operator   string            `json:"operator"`
	Conditions []WhereExpression `json:"conditions,omitempty"`
	Left       *WhereOperand     `json:"left,omitempty"`
	Right      *WhereOperand     `json:"right,omitempty"`
}

// WhereOperand is either a column reference or a constant value.
type WhereOperand struct {
	Column   string      `json:"column,omitempty"`
	Constant interface{} `json:"constant,omitempty"`
}

// RecordsResponse is the response from POST /records/query.
type RecordsResponse struct {
	Columns []string        `json:"columns"`
	Items   [][]interface{} `json:"items"`
}

// InfoResponse is the response from GET /info. It advertises the active
// provider and — on capability-aware DataBridge images — the set of
// aggregations and stats that provider can actually compute.
//
// Capabilities is nullable: older DataBridge images omit it. A nil value means
// "unknown", and consumers must degrade open (offer everything).
type InfoResponse struct {
	Edition            string            `json:"edition"`
	ActiveProvider     string            `json:"activeProvider"`
	AvailableProviders []string          `json:"availableProviders"`
	Capabilities       *CapabilitiesInfo `json:"capabilities,omitempty"`
}

// CapabilitiesInfo describes what the active provider supports. The sets use
// the DataBridge vocabularies: SupportedAggregations is the query-time SELECT
// function set, SupportedStats is the ComputeStats set.
type CapabilitiesInfo struct {
	SupportedAggregations           []string `json:"supportedAggregations"`
	SupportedStats                  []string `json:"supportedStats"`
	SupportsExactComputeOnRawWindow bool     `json:"supportsExactComputeOnRawWindow"`
}

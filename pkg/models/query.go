package models

// QueryDefinition represents the JSON query sent from the frontend.
type QueryDefinition struct {
	Mode             string             `json:"mode"`
	Strategy         string             `json:"strategy"`
	OptimizeDisplay  bool               `json:"optimizeDisplay"`
	CatalogEntryIds  []string           `json:"catalogEntryIds,omitempty"`
	ConnectionId     string             `json:"connectionId,omitempty"`
	DatabaseName     string             `json:"databaseName,omitempty"`
	DatasetName      string             `json:"datasetName,omitempty"`
	Select           []SelectDefinition `json:"select"`
	Where            *FilterDefinition  `json:"where,omitempty"`
	Aggregation      string             `json:"aggregation,omitempty"`
	TimeWindowSeconds int               `json:"timeWindowSeconds,omitempty"`
	Limit            int                `json:"limit,omitempty"`
	Offset           int                `json:"offset,omitempty"`
	OrderByColumn    string             `json:"orderByColumn,omitempty"`
	OrderByDirection string             `json:"orderByDirection,omitempty"`
	DisplayNamePreset  string           `json:"displayNamePreset,omitempty"`
	DisplayNamePattern string           `json:"displayNamePattern,omitempty"`
}

// SelectDefinition represents a column selection with aggregation.
type SelectDefinition struct {
	CatalogEntryId     string `json:"catalogEntryId,omitempty"`
	Column             string `json:"column,omitempty"`
	Aggregation        string `json:"aggregation,omitempty"`
	Alias              string `json:"alias,omitempty"`
	DisplayNamePreset  string `json:"displayNamePreset,omitempty"`
	DisplayNamePattern string `json:"displayNamePattern,omitempty"`
}

// FilterDefinition is a recursive filter tree: either a logical group (AND/OR)
// containing sub-filters, or a single comparison condition.
// Discriminated by the presence of "conditions" (logical) vs "column" (comparison).
type FilterDefinition struct {
	// Logical group fields
	Operator   string             `json:"operator"`             // "and" | "or" for groups, comparison op for leaf
	Conditions []FilterDefinition `json:"conditions,omitempty"` // sub-filters (logical group only)

	// Comparison leaf fields
	Column string      `json:"column,omitempty"`
	Value  interface{} `json:"value,omitempty"`
}

// IsLogicalGroup returns true if this is an AND/OR group with sub-conditions.
func (f *FilterDefinition) IsLogicalGroup() bool {
	return len(f.Conditions) > 0
}

// WhereCondition represents a flat filter condition (legacy format, still supported).
type WhereCondition struct {
	Column   string      `json:"column"`
	Operator string      `json:"operator"`
	Value    interface{} `json:"value"`
}

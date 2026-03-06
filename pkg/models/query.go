package models

import "encoding/json"

// QueryDefinition represents the JSON query sent from the frontend.
type QueryDefinition struct {
	Mode              string             `json:"mode"`
	Strategy          string             `json:"strategy"`
	OptimizeDisplay   bool               `json:"optimizeDisplay"`
	CatalogEntryIds   []string           `json:"catalogEntryIds,omitempty"`
	ConnectionId      string             `json:"connectionId,omitempty"`
	DatabaseName      string             `json:"databaseName,omitempty"`
	DatasetName       string             `json:"datasetName,omitempty"`
	Select            []SelectDefinition `json:"select"`
	Where             *FilterDefinition  `json:"-"` // custom unmarshal
	WhereRaw          json.RawMessage    `json:"where,omitempty"`
	Aggregation       string             `json:"aggregation,omitempty"`
	TimeWindowSeconds int                `json:"timeWindowSeconds,omitempty"`
	Limit             int                `json:"limit,omitempty"`
	Offset            int                `json:"offset,omitempty"`
	OrderByColumn     string             `json:"orderByColumn,omitempty"`
	OrderByDirection  string             `json:"orderByDirection,omitempty"`
	DisplayNamePreset  string            `json:"displayNamePreset,omitempty"`
	DisplayNamePattern string            `json:"displayNamePattern,omitempty"`
}

// ParseWhere converts the raw JSON where field into a FilterDefinition.
// Supports both legacy array format and new object format.
func (qd *QueryDefinition) ParseWhere() {
	if len(qd.WhereRaw) == 0 {
		return
	}

	raw := []byte(qd.WhereRaw)

	// Try new format (object with operator + conditions or column)
	var filter FilterDefinition
	if err := json.Unmarshal(raw, &filter); err == nil && filter.Operator != "" {
		qd.Where = &filter
		return
	}

	// Try legacy format (array of flat conditions)
	var legacy []WhereCondition
	if err := json.Unmarshal(raw, &legacy); err == nil && len(legacy) > 0 {
		conditions := make([]FilterDefinition, 0, len(legacy))
		for _, c := range legacy {
			conditions = append(conditions, FilterDefinition{
				Column:   c.Column,
				Operator: c.Operator,
				Value:    c.Value,
			})
		}
		qd.Where = &FilterDefinition{
			Operator:   "and",
			Conditions: conditions,
		}
	}
}

// SelectDefinition represents a column selection with aggregation.
type SelectDefinition struct {
	CatalogEntryId     string `json:"catalogEntryId,omitempty"`
	Column             string `json:"column,omitempty"`
	DataType           string `json:"dataType,omitempty"`
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

// WhereCondition represents a flat filter condition (legacy format).
type WhereCondition struct {
	Column   string      `json:"column"`
	Operator string      `json:"operator"`
	Value    interface{} `json:"value"`
}

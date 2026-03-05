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
	Where            []WhereCondition   `json:"where,omitempty"`
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

// WhereCondition represents a filter condition.
type WhereCondition struct {
	Column   string      `json:"column"`
	Operator string      `json:"operator"`
	Value    interface{} `json:"value"`
}

package datacatalog

// SourceConnection represents a DataCatalog source connection.
type SourceConnection struct {
	ID           string      `json:"id"`
	Name         string      `json:"name"`
	SourceTypeID string      `json:"sourceTypeId,omitempty"`
	SourceType   *SourceType `json:"sourceType,omitempty"`
	URL          string      `json:"url"`
}

// SourceType represents a source type (e.g. DataBridge, InfluxDB).
type SourceType struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// CatalogEntry represents a catalog entry from the DataCatalog API.
type CatalogEntry struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	SourceConnection *SourceConnection `json:"sourceConnection,omitempty"`
	SourceConnectionID string          `json:"sourceConnectionId,omitempty"`
	DataType         string            `json:"dataType"`
	Labels           []Label           `json:"labels"`
	Metadata         *CatalogMetadata  `json:"metadata,omitempty"`
	SourceParams     map[string]string `json:"sourceParams"`
}

// GetSourceConnectionID returns the source connection ID from either the nested object or the flat field.
func (e *CatalogEntry) GetSourceConnectionID() string {
	if e.SourceConnection != nil {
		return e.SourceConnection.ID
	}
	return e.SourceConnectionID
}

// GetLabelNames returns the label names as a string slice.
func (e *CatalogEntry) GetLabelNames() []string {
	names := make([]string, len(e.Labels))
	for i, l := range e.Labels {
		names[i] = l.Name
	}
	return names
}

// CatalogMetadata holds optional metadata for a catalog entry.
type CatalogMetadata struct {
	TagLevel1   string            `json:"tagLevel1,omitempty"`
	Description map[string]string `json:"description,omitempty"`
	Unit        string            `json:"unit,omitempty"`
	Min         *float64          `json:"min,omitempty"`
	Max         *float64          `json:"max,omitempty"`
	Decimals    *int              `json:"decimals,omitempty"`
	Scale       *float64          `json:"scale,omitempty"`
}

// AssetDictionary represents a tree-structured asset hierarchy.
type AssetDictionary struct {
	ID    string      `json:"id"`
	Name  string      `json:"name"`
	Nodes []AssetNode `json:"nodes"`
}

// AssetNode is a node in an asset dictionary tree.
type AssetNode struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	ParentID   *string     `json:"parentId"`
	Children   []AssetNode `json:"children"`
	EntryIds   []string    `json:"entryIds,omitempty"`
	EntryCount int         `json:"entryCount"`
}

// Label represents a DataCatalog label.
type Label struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// PaginatedResponse wraps paginated API responses.
type PaginatedResponse[T any] struct {
	Items      []T `json:"items"`
	TotalCount int `json:"totalCount"`
}

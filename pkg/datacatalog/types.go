package datacatalog

// SourceConnection represents a DataCatalog source connection.
type SourceConnection struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	SourceTypeID string `json:"sourceTypeId"`
	URL          string `json:"url"`
}

// CatalogEntry represents a catalog entry from the DataCatalog API.
type CatalogEntry struct {
	ID                 string              `json:"id"`
	Name               string              `json:"name"`
	SourceConnectionID string              `json:"sourceConnectionId"`
	DataType           string              `json:"dataType"`
	Labels             []string            `json:"labels"`
	Metadata           *CatalogMetadata    `json:"metadata"`
	SourceParams       map[string]string   `json:"sourceParams"`
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
	Children   []AssetNode `json:"children,omitempty"`
	EntryCount int         `json:"entryCount,omitempty"`
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

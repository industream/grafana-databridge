package datacatalog

import (
	"encoding/json"
	"strconv"
)

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
	SourceParams     map[string]any `json:"sourceParams"`
}

// GetSourceParam returns a source parameter as a string, or empty string if not found or not a string.
func (e *CatalogEntry) GetSourceParam(key string) string {
	if v, ok := e.SourceParams[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// GetSourceConnectionID returns the source connection ID from either the nested object or the flat field.
func (e *CatalogEntry) GetSourceConnectionID() string {
	if e.SourceConnection != nil {
		return e.SourceConnection.ID
	}
	return e.SourceConnectionID
}

// IsDataBridgeEntry returns true if this entry belongs to a DataBridge source connection.
func (e *CatalogEntry) IsDataBridgeEntry() bool {
	if e.SourceConnection == nil || e.SourceConnection.SourceType == nil {
		return false
	}
	return e.SourceConnection.SourceType.ID == "DataBridge"
}

// GetLabelNames returns the label names as a string slice.
func (e *CatalogEntry) GetLabelNames() []string {
	names := make([]string, len(e.Labels))
	for i, l := range e.Labels {
		names[i] = l.Name
	}
	return names
}

// FlexFloat64 handles JSON values that can be either a number or a string containing a number.
type FlexFloat64 struct {
	Value *float64
}

func (f FlexFloat64) MarshalJSON() ([]byte, error) {
	if f.Value == nil {
		return json.Marshal(nil)
	}
	return json.Marshal(*f.Value)
}

func (f *FlexFloat64) UnmarshalJSON(data []byte) error {
	var num float64
	if err := json.Unmarshal(data, &num); err == nil {
		f.Value = &num
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		if s == "" {
			f.Value = nil
			return nil
		}
		num, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil // silently ignore unparseable values
		}
		f.Value = &num
		return nil
	}
	return nil
}

// FlexInt handles JSON values that can be either a number or a string containing an integer.
type FlexInt struct {
	Value *int
}

func (f FlexInt) MarshalJSON() ([]byte, error) {
	if f.Value == nil {
		return json.Marshal(nil)
	}
	return json.Marshal(*f.Value)
}

func (f *FlexInt) UnmarshalJSON(data []byte) error {
	var num int
	if err := json.Unmarshal(data, &num); err == nil {
		f.Value = &num
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		if s == "" {
			f.Value = nil
			return nil
		}
		num, err := strconv.Atoi(s)
		if err != nil {
			return nil
		}
		f.Value = &num
		return nil
	}
	return nil
}

// FlexString handles JSON values that can be either a string or a number,
// always rendered as a string (DataCatalog may return e.g. `unit` as either).
type FlexString string

func (f FlexString) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(f))
}

func (f *FlexString) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*f = FlexString(s)
		return nil
	}
	var num float64
	if err := json.Unmarshal(data, &num); err == nil {
		*f = FlexString(strconv.FormatFloat(num, 'f', -1, 64))
		return nil
	}
	return nil // silently ignore unparseable values
}

// CatalogMetadata holds optional metadata for a catalog entry.
// Unit, Min, Max, Decimals, Scale use Flex types because DataCatalog may return them as strings or numbers.
type CatalogMetadata struct {
	TagLevel1        string            `json:"tagLevel1,omitempty"`
	DescriptionMap   map[string]string `json:"-"`
	DescriptionRaw   json.RawMessage   `json:"description,omitempty"`
	Unit             FlexString        `json:"unit,omitempty"`
	Min              FlexFloat64       `json:"min,omitempty"`
	Max              FlexFloat64       `json:"max,omitempty"`
	Decimals         FlexInt           `json:"decimals,omitempty"`
	Scale            FlexFloat64       `json:"scale,omitempty"`
}

// Description returns the description as a localized map.
// Handles both string ("some text") and object ({"en-US": "text"}) formats.
func (m *CatalogMetadata) Description() map[string]string {
	if m.DescriptionMap != nil {
		return m.DescriptionMap
	}
	if len(m.DescriptionRaw) == 0 {
		return nil
	}

	// Try as map first
	var asMap map[string]string
	if err := json.Unmarshal(m.DescriptionRaw, &asMap); err == nil {
		m.DescriptionMap = asMap
		return asMap
	}

	// Fall back to plain string
	var asString string
	if err := json.Unmarshal(m.DescriptionRaw, &asString); err == nil {
		m.DescriptionMap = map[string]string{"": asString}
		return m.DescriptionMap
	}

	return nil
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

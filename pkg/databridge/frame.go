package databridge

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/data"
)

// ToDataFrame converts a RecordsResponse into a Grafana data.Frame.
// Column types are inferred from the first non-nil value in each column.
func ToDataFrame(name string, resp *RecordsResponse) (*data.Frame, error) {
	if resp == nil || len(resp.Columns) == 0 {
		return data.NewFrame(name), nil
	}

	rowCount := len(resp.Items)
	colCount := len(resp.Columns)

	// Infer column types from the first non-nil value
	colTypes := make([]string, colCount)
	for colIdx := range resp.Columns {
		for _, row := range resp.Items {
			if colIdx < len(row) && row[colIdx] != nil {
				colTypes[colIdx] = inferType(resp.Columns[colIdx], row[colIdx])
				break
			}
		}
		if colTypes[colIdx] == "" {
			colTypes[colIdx] = "string" // fallback
		}
	}

	fields := make([]*data.Field, colCount)
	for i, colName := range resp.Columns {
		fields[i] = createField(colName, colTypes[i], rowCount)
	}

	for rowIdx, row := range resp.Items {
		for colIdx := range resp.Columns {
			if colIdx >= len(row) {
				continue
			}
			if err := setFieldValue(fields[colIdx], rowIdx, colTypes[colIdx], row[colIdx]); err != nil {
				return nil, fmt.Errorf("row %d col %q: %w", rowIdx, resp.Columns[colIdx], err)
			}
		}
	}

	frame := data.NewFrame(name, fields...)
	return frame, nil
}

// inferType determines the column type from the column name and a sample value.
func inferType(colName string, value interface{}) string {
	// Time columns are detected by name
	lower := strings.ToLower(colName)
	if lower == "time" || lower == "timestamp" || lower == "bucket" || strings.HasSuffix(lower, "_time") {
		if _, ok := value.(string); ok {
			return "datetime"
		}
	}

	switch value.(type) {
	case bool:
		return "bool"
	case float64:
		return "float64"
	case string:
		// Try parsing as time
		if _, err := parseTime(value); err == nil {
			return "datetime"
		}
		return "string"
	default:
		return "string"
	}
}

func createField(colName, dataType string, rowCount int) *data.Field {
	switch dataType {
	case "datetime":
		return data.NewField(colName, nil, make([]*time.Time, rowCount))
	case "float64":
		return data.NewField(colName, nil, make([]*float64, rowCount))
	case "int64":
		return data.NewField(colName, nil, make([]*int64, rowCount))
	case "bool":
		return data.NewField(colName, nil, make([]*bool, rowCount))
	default:
		return data.NewField(colName, nil, make([]*string, rowCount))
	}
}

func setFieldValue(field *data.Field, rowIdx int, dataType string, value interface{}) error {
	if value == nil {
		return nil
	}

	switch dataType {
	case "datetime":
		t, err := parseTime(value)
		if err != nil {
			return err
		}
		field.Set(rowIdx, t)
	case "float64":
		v := toFloat64(value)
		field.Set(rowIdx, v)
	case "int64":
		v := toInt64(value)
		field.Set(rowIdx, v)
	case "bool":
		v := toBool(value)
		field.Set(rowIdx, v)
	default:
		s := fmt.Sprintf("%v", value)
		field.Set(rowIdx, &s)
	}
	return nil
}


func parseTime(value interface{}) (*time.Time, error) {
	switch v := value.(type) {
	case string:
		for _, layout := range []string{
			time.RFC3339Nano,
			time.RFC3339,
			"2006-01-02T15:04:05",
			"2006-01-02 15:04:05",
		} {
			if t, err := time.Parse(layout, v); err == nil {
				return &t, nil
			}
		}
		return nil, fmt.Errorf("cannot parse time %q", v)
	case float64:
		// Unix timestamp in seconds or milliseconds
		if v > 1e12 {
			t := time.UnixMilli(int64(v))
			return &t, nil
		}
		t := time.Unix(int64(v), 0)
		return &t, nil
	default:
		return nil, fmt.Errorf("unsupported time type %T", value)
	}
}

func toFloat64(value interface{}) *float64 {
	switch v := value.(type) {
	case float64:
		if math.IsNaN(v) {
			return nil
		}
		return &v
	case float32:
		f := float64(v)
		return &f
	case int:
		f := float64(v)
		return &f
	case int64:
		f := float64(v)
		return &f
	case json_number:
		f, err := v.Float64()
		if err != nil {
			return nil
		}
		return &f
	default:
		return nil
	}
}

// json_number is a type alias to avoid importing encoding/json here.
// In practice, json.Number comes from JSON decoding with UseNumber.
type json_number = interface{ Float64() (float64, error) }

func toInt64(value interface{}) *int64 {
	switch v := value.(type) {
	case float64:
		i := int64(v)
		return &i
	case int:
		i := int64(v)
		return &i
	case int64:
		return &v
	default:
		return nil
	}
}

func toBool(value interface{}) *bool {
	switch v := value.(type) {
	case bool:
		return &v
	case float64:
		b := v != 0
		return &b
	case string:
		b := strings.ToLower(v) == "true" || v == "1"
		return &b
	default:
		return nil
	}
}

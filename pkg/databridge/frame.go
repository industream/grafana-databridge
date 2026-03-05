package databridge

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/data"
)

// ToDataFrame converts a RecordsResponse into a Grafana data.Frame.
func ToDataFrame(name string, resp *RecordsResponse) (*data.Frame, error) {
	if resp == nil || len(resp.Columns) == 0 {
		return data.NewFrame(name), nil
	}

	fields := make([]*data.Field, len(resp.Columns))
	for i, col := range resp.Columns {
		field, err := createField(col, len(resp.Items))
		if err != nil {
			return nil, fmt.Errorf("create field %q: %w", col.Name, err)
		}
		fields[i] = field
	}

	for rowIdx, row := range resp.Items {
		for colIdx, col := range resp.Columns {
			if colIdx >= len(row) {
				continue
			}
			if err := setFieldValue(fields[colIdx], rowIdx, col.DataType, row[colIdx]); err != nil {
				return nil, fmt.Errorf("row %d col %q: %w", rowIdx, col.Name, err)
			}
		}
	}

	frame := data.NewFrame(name, fields...)
	return frame, nil
}

func createField(col RecordsColumn, rowCount int) (*data.Field, error) {
	dt := normalizeDataType(col.DataType)
	switch dt {
	case "datetime", "timestamp":
		values := make([]*time.Time, rowCount)
		return data.NewField(col.Name, nil, values), nil
	case "float64", "float32", "double", "decimal", "numeric":
		values := make([]*float64, rowCount)
		return data.NewField(col.Name, nil, values), nil
	case "int64", "int32", "int16", "integer", "bigint", "smallint":
		values := make([]*int64, rowCount)
		return data.NewField(col.Name, nil, values), nil
	case "bool", "boolean":
		values := make([]*bool, rowCount)
		return data.NewField(col.Name, nil, values), nil
	case "string", "text", "varchar":
		values := make([]*string, rowCount)
		return data.NewField(col.Name, nil, values), nil
	default:
		values := make([]*string, rowCount)
		return data.NewField(col.Name, nil, values), nil
	}
}

func setFieldValue(field *data.Field, rowIdx int, dataType string, value interface{}) error {
	if value == nil {
		return nil // nullable field, leave as nil
	}

	dt := normalizeDataType(dataType)
	switch dt {
	case "datetime", "timestamp":
		t, err := parseTime(value)
		if err != nil {
			return err
		}
		field.Set(rowIdx, t)
	case "float64", "float32", "double", "decimal", "numeric":
		v := toFloat64(value)
		field.Set(rowIdx, v)
	case "int64", "int32", "int16", "integer", "bigint", "smallint":
		v := toInt64(value)
		field.Set(rowIdx, v)
	case "bool", "boolean":
		v := toBool(value)
		field.Set(rowIdx, v)
	default:
		s := fmt.Sprintf("%v", value)
		field.Set(rowIdx, &s)
	}
	return nil
}

func normalizeDataType(dt string) string {
	return strings.ToLower(strings.TrimSpace(dt))
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
